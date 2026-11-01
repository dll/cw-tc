package api

import (
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminCreateStaff POST /admin/staff —— 开通教师/管理员（只有 super 能建 super）。
func (s *Server) adminCreateStaff(c *gin.Context) {
	var req struct {
		Account  string `json:"account"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	// 教师账号：ops + super 可开通；管理员账号：仅 super 可开通（PRD §7.10.10）。
	if req.Role != string(staff.RoleTeacher) && admin.Role != staff.RoleAdminSuper {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权限"})
		return
	}
	if staff.Role(req.Role) == staff.RoleAdminSuper && admin.Role != staff.RoleAdminSuper {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权限"})
		return
	}
	id, err := staff.Create(c.Request.Context(), s.dbConn, req.Account, req.Name, staff.Role(req.Role), req.Password, admin.ID)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "staff.create", TargetType: "staff", TargetID: strconv.FormatInt(id, 10),
		After: `{"role":"` + req.Role + `"}`, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

// adminDisableStaff POST /admin/staff/:id/disable —— 停用（教师由 ops/super；管理员仅 super）。
func (s *Server) adminDisableStaff(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var role string
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT role FROM staff WHERE id = $1`, id).Scan(&role); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "账号不存在"})
		return
	}
	if (role == string(staff.RoleAdminSuper) || role == string(staff.RoleAdminOps)) && admin.Role != staff.RoleAdminSuper {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "仅超级管理员可停用管理员账号"})
		return
	}
	if role == string(staff.RoleAdminSuper) {
		if id == admin.ID {
			c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "不能停用自己"})
			return
		}
		var supers int
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT COUNT(*) FROM staff WHERE role = 'admin_super' AND status = 'active'`).Scan(&supers); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		if supers <= 1 {
			c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "不能停用最后一个超管"})
			return
		}
	}
	if err := staff.Disable(c.Request.Context(), s.dbConn, id); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "staff.disable", TargetType: "staff", TargetID: strconv.FormatInt(id, 10),
		After: `{"status":"disabled","role":"` + role + `"}`, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminTransferCourse POST /admin/courses/transfer —— 课程移交给另一位在职教师。
func (s *Server) adminTransferCourse(c *gin.Context) {
	var req struct {
		CourseID    int64 `json:"course_id"`
		ToTeacherID int64 `json:"to_teacher_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CourseID <= 0 || req.ToTeacherID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if err := staff.TransferCourse(c.Request.Context(), s.dbConn, req.CourseID, req.ToTeacherID); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminRefundOrder POST /admin/orders/:orderNo/refund —— 管理员退款（二次确认 + 写审计）。
func (s *Server) adminRefundOrder(c *gin.Context) {
	var req struct {
		Kind        string `json:"kind"`
		Basis       string `json:"basis"`
		BasisRef    string `json:"basis_ref"`
		AmountCents int64  `json:"amount_cents"`
		Reason      string `json:"reason"`
		Confirm     bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "高危操作需二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	ref, err := commerce.NewService(s.dbConn).Refund(c.Request.Context(), commerce.RefundInput{
		OrderNo: c.Param("orderNo"), Kind: req.Kind, Basis: req.Basis, BasisRef: req.BasisRef,
		AmountCents: req.AmountCents, Reason: req.Reason, Initiator: "admin", OperatorID: admin.ID,
	})
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	s.notifyOrderRefunded(c.Request.Context(), c.Param("orderNo"))
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"refund_no": ref.RefundNo}})
}
