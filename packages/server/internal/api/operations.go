package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/course"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

type operationSlotRequest struct {
	Position   string `json:"position"`
	CourseCode string `json:"course_code"`
	Title      string `json:"title"`
	ImageURL   string `json:"image_url"`
	LinkURL    string `json:"link_url"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	Weight     int    `json:"weight"`
	Status     string `json:"status"`
}

func (in operationSlotRequest) slot() course.OperationSlot {
	return course.OperationSlot{
		Position: in.Position, CourseCode: in.CourseCode, Title: in.Title,
		ImageURL: in.ImageURL, LinkURL: in.LinkURL, StartsAt: in.StartsAt,
		EndsAt: in.EndsAt, Weight: in.Weight, Status: in.Status,
	}
}

func operationSlotDTO(item course.OperationSlot) gin.H {
	return gin.H{
		"id": item.ID, "position": item.Position, "course_code": item.CourseCode,
		"title": item.Title, "image_url": item.ImageURL, "link_url": item.LinkURL,
		"starts_at": item.StartsAt, "ends_at": item.EndsAt, "weight": item.Weight,
		"status": item.Status, "created_by": item.CreatedBy,
	}
}

// adminListOperations GET /admin/operations —— 运营位列表（含草稿 / 已过期 / 已移除）。
func (s *Server) adminListOperations(c *gin.Context) {
	items, err := course.ListOperationSlots(c.Request.Context(), s.dbConn, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "运营位服务不可用"})
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, operationSlotDTO(item))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": data}})
}

// adminCreateOperation POST /admin/operations —— 新建运营位。
func (s *Server) adminCreateOperation(c *gin.Context) {
	var req operationSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	id, err := course.CreateOperationSlot(c.Request.Context(), s.dbConn, req.slot(), admin.ID)
	if err != nil {
		operationSlotError(c, err)
		return
	}
	after, _ := json.Marshal(map[string]any{"position": req.Position, "course_code": req.CourseCode, "status": req.Status})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "operation_slot.create", TargetType: "operation_slot", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

// adminUpdateOperation PATCH /admin/operations/:id —— 更新运营位。
func (s *Server) adminUpdateOperation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req operationSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if err := course.UpdateOperationSlot(c.Request.Context(), s.dbConn, id, req.slot(), admin.ID); err != nil {
		operationSlotError(c, err)
		return
	}
	after, _ := json.Marshal(map[string]any{"position": req.Position, "status": req.Status})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "operation_slot.update", TargetType: "operation_slot", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminRemoveOperation DELETE /admin/operations/:id —— 下线运营位（保留记录）。
func (s *Server) adminRemoveOperation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if err := course.RemoveOperationSlot(c.Request.Context(), s.dbConn, id); err != nil {
		operationSlotError(c, err)
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "operation_slot.remove", TargetType: "operation_slot", TargetID: strconv.FormatInt(id, 10),
		After: "removed", IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// publicOperations GET /public/operations?position=home_banner —— 当前生效运营位。
func (s *Server) publicOperations(c *gin.Context) {
	position := c.Query("position")
	items, err := course.PublicOperationSlots(c.Request.Context(), s.dbConn, position, time.Now().UTC())
	if err != nil {
		if err == course.ErrSlotInput {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "position 参数非法"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "运营位服务不可用"})
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, operationSlotDTO(item))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": data}})
}

func operationSlotError(c *gin.Context, err error) {
	switch err {
	case course.ErrSlotInput:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "运营位参数非法（时间/位置/课程状态）"})
	case course.ErrSlotNotFound:
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "运营位不存在"})
	case course.ErrCourseNotFound:
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
	default:
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "运营位服务不可用"})
	}
}
