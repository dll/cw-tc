package api

import (
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/course"
	"github.com/cw-tc/server/internal/notify"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminGrantEntitlement POST /admin/entitlements —— 授权免费学习（事由必填，二次确认在前端）。
func (s *Server) adminGrantEntitlement(c *gin.Context) {
	var req struct {
		StudentID   int64  `json:"student_id"`
		CourseID    int64  `json:"course_id"`
		CourseCode  string `json:"course_code"`
		GrantReason string `json:"grant_reason"`
		Confirm bool `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.StudentID <= 0 || req.GrantReason == "" || !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	// 允许用公开标识 course_code 授权（客户端只有课程码，没有内部 id）
	if req.CourseID <= 0 && req.CourseCode != "" {
		var cid int64
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT id FROM course WHERE course_code = $1`, req.CourseCode).Scan(&cid); err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "课程不存在"})
			return
		}
		req.CourseID = cid
	}
	if req.CourseID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if err := commerce.NewService(s.dbConn).GrantEntitlement(c.Request.Context(), req.StudentID, req.CourseID, admin.ID, req.GrantReason); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminRevokeEntitlement DELETE /admin/entitlements/:studentID/:courseID —— 撤销授权（只收权益、证书保留）。
func (s *Server) adminRevokeEntitlement(c *gin.Context) {
	studentID, err1 := strconv.ParseInt(c.Param("studentID"), 10, 64)
	courseID, err2 := strconv.ParseInt(c.Param("courseID"), 10, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if err := commerce.NewService(s.dbConn).RevokeGrant(c.Request.Context(), studentID, courseID); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminReviewCourse POST /admin/courses/:code/review —— 六维审核（驳回必填问题清单）。
func (s *Server) adminReviewCourse(c *gin.Context) {
	var req struct {
		ContentVersion  int     `json:"content_version"`
		ScoreMaterial   float64 `json:"score_material"`
		ScoreObjective  float64 `json:"score_objective"`
		ScoreCase       float64 `json:"score_case"`
		ScoreLessonPlan float64 `json:"score_lesson_plan"`
		ScoreSlides     float64 `json:"score_slides"`
		ScoreLab        float64 `json:"score_lab"`
		DimensionNotes  string  `json:"dimension_notes"`
		IssueList       string  `json:"issue_list"`
		Verdict         string  `json:"verdict"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	err := course.ReviewCourse(c.Request.Context(), s.dbConn, course.ReviewInput{
		CourseCode:      c.Param("code"),
		ContentVersion:  req.ContentVersion,
		ReviewerID:      admin.ID,
		ReviewerRole:    string(admin.Role),
		ScoreMaterial:   req.ScoreMaterial,
		ScoreObjective:  req.ScoreObjective,
		ScoreCase:       req.ScoreCase,
		ScoreLessonPlan: req.ScoreLessonPlan,
		ScoreSlides:     req.ScoreSlides,
		ScoreLab:        req.ScoreLab,
		DimensionNotes:  req.DimensionNotes,
		IssueList:       req.IssueList,
		Verdict:         req.Verdict,
	})
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminPublishCourse POST /admin/courses/:code/publish —— 上架。
func (s *Server) adminPublishCourse(c *gin.Context) {
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if err := course.PublishCourse(c.Request.Context(), s.dbConn, c.Param("code"), admin.ID); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminSetPrice POST /admin/courses/:code/price —— 改价（二次确认 + 写审计，铁律 8）。
func (s *Server) adminSetPrice(c *gin.Context) {
	var req struct {
		PriceCents int64 `json:"price_cents"`
		Confirm    bool  `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PriceCents < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "高危操作需二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if err := course.SetPrice(c.Request.Context(), s.dbConn, c.Param("code"), req.PriceCents); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.price_change", TargetType: "course", TargetID: c.Param("code"),
		After: strconv.FormatInt(req.PriceCents, 10), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminCreateBroadcast POST /admin/broadcasts —— 广播草稿（二次确认第一步）。
func (s *Server) adminCreateBroadcast(c *gin.Context) {
	var req struct {
		Target         string `json:"target"`
		TargetValue    string `json:"target_value"`
		Title          string `json:"title"`
		Body           string `json:"body"`
		Link           string `json:"link"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	id, err := notify.CreateBroadcast(c.Request.Context(), s.dbConn, notify.BroadcastInput{
		OperatorID: admin.ID, Target: req.Target, TargetValue: req.TargetValue,
		Title: req.Title, Body: req.Body, Link: req.Link, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"broadcast_id": id}})
}

// adminConfirmBroadcast POST /admin/broadcasts/:id/confirm —— 二次确认发送。
func (s *Server) adminConfirmBroadcast(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	n, err := notify.ConfirmBroadcast(c.Request.Context(), s.dbConn, id, admin.ID)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"recipients": n}})
}
