package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/course"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func contentRequestDTO(item course.ContentRequest) gin.H {
	return gin.H{
		"id": item.ID, "course_code": item.CourseCode, "course_name": item.CourseName,
		"teacher_id": item.TeacherID, "teacher_name": item.TeacherName,
		"kind": item.Kind, "reason_kind": item.ReasonKind, "reason_text": item.ReasonText,
		"status": item.Status, "handled_by": item.HandledBy, "handled_at": item.HandledAt,
		"created_at": item.CreatedAt,
	}
}

// teacherCreateContentRequest POST /teacher/courses/:code/content-requests —— 下架 / 紧急纠错申请。
func (s *Server) teacherCreateContentRequest(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	var req struct {
		Kind       string `json:"kind"`
		ReasonKind string `json:"reason_kind"`
		ReasonText string `json:"reason_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	id, err := course.CreateContentRequest(c.Request.Context(), s.dbConn, course.ContentRequest{
		CourseCode: c.Param("code"), TeacherID: st.ID,
		Kind: req.Kind, ReasonKind: req.ReasonKind, ReasonText: req.ReasonText,
	})
	if err != nil {
		switch err {
		case course.ErrCourseNotFound:
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在或不属于本人"})
		case course.ErrInvalidTransition:
			c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "只有已上架课程可申请下架 / 紧急纠错"})
		case course.ErrContentRequestExists:
			c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		case course.ErrContentRequestInput:
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "申请类型或原因非法"})
		default:
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申请失败，请稍后重试"})
		}
		return
	}
	after, _ := json.Marshal(map[string]string{"kind": req.Kind, "reason_kind": req.ReasonKind})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role),
		Action: "content_request.create", TargetType: "course", TargetID: c.Param("code"),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": "pending"}})
}

// teacherListContentRequests GET /teacher/content-requests —— 我的申请记录。
func (s *Server) teacherListContentRequests(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	items, err := course.ListContentRequests(c.Request.Context(), s.dbConn, c.Query("status"), st.ID, 100, 0)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申请列表暂不可用"})
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, contentRequestDTO(item))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": data}})
}

// adminListContentRequests GET /admin/content-requests —— 管理员待办。
func (s *Server) adminListContentRequests(c *gin.Context) {
	items, err := course.ListContentRequests(c.Request.Context(), s.dbConn, c.Query("status"), 0, 100, 0)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申请列表暂不可用"})
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, contentRequestDTO(item))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": data}})
}

// adminApproveContentRequest POST /admin/content-requests/:id/approve —— 批准并下架。
func (s *Server) adminApproveContentRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "需要二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var courseStatus string
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT c.status FROM content_request r JOIN course c ON c.course_code = r.course_code WHERE r.id = $1`, id).Scan(&courseStatus)
	handled, err := course.HandleContentRequest(c.Request.Context(), s.dbConn, id, admin.ID, true)
	if err != nil {
		switch err {
		case course.ErrContentRequestNotFound:
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "申请不存在"})
		case course.ErrInvalidTransition:
			c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "申请已处理"})
		default:
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "处理失败，请稍后重试"})
		}
		return
	}
	after, _ := json.Marshal(map[string]string{"kind": handled.Kind, "course_code": handled.CourseCode})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "content_request.approve", TargetType: "content_request", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	if courseStatus == "published" {
		unpublishAfter, _ := json.Marshal(map[string]string{
			"from": "published", "to": "unpublished", "reason": handled.ReasonKind,
		})
		_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
			ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
			Action: "course.unpublish", TargetType: "course", TargetID: handled.CourseCode,
			After: string(unpublishAfter), IP: c.ClientIP(),
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "approved"}})
}

// adminRejectContentRequest POST /admin/content-requests/:id/reject —— 驳回申请。
func (s *Server) adminRejectContentRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || !req.Confirm || strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "驳回事由必填且需二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	handled, err := course.HandleContentRequest(c.Request.Context(), s.dbConn, id, admin.ID, false)
	if err != nil {
		switch err {
		case course.ErrContentRequestNotFound:
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "申请不存在"})
		case course.ErrInvalidTransition:
			c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "申请已处理"})
		default:
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "处理失败，请稍后重试"})
		}
		return
	}
	after, _ := json.Marshal(map[string]string{"course_code": handled.CourseCode, "reason": req.Reason})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "content_request.reject", TargetType: "content_request", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "rejected"}})
}
