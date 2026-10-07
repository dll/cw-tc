package api

import (
	"net/http"
	"strconv"

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
	if staff.Role(req.Role) == staff.RoleAdminSuper && admin.Role != staff.RoleAdminSuper {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权限"})
		return
	}
	id, err := staff.Create(c.Request.Context(), s.dbConn, req.Account, req.Name, staff.Role(req.Role), req.Password, admin.ID)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

// adminDisableStaff POST /admin/staff/:id/disable —— 停用（名下课程未移交则拒）。
func (s *Server) adminDisableStaff(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if err := staff.Disable(c.Request.Context(), s.dbConn, id); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
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
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"refund_no": ref.RefundNo}})
}
