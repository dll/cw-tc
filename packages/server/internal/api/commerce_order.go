package api

import (
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) createOrder(c *gin.Context) {
	studentValue, exists := c.Get(studentContextKey)
	if !exists || s.dbConn == nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	student := studentValue.(identity.Student)
	var input struct {
		CourseCode string `json:"course_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.CourseCode == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	// 客户端只有课程码，后端解析 course_id（与开考 POST /exams 同口径）。
	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT id FROM course WHERE course_code = $1 AND status = 'published'`, input.CourseCode).Scan(&courseID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在或未上架"})
		return
	}
	order, err := commerce.NewService(s.dbConn).CreateOrderFromCourse(c.Request.Context(), student.ID, courseID, c.GetHeader("Idempotency-Key"), time.Now().UTC().Add(30*time.Minute))
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"order_no": order.OrderNo, "status": order.Status, "amount_cents": order.AmountCents}})
}

// mockPayOrder POST /orders/:orderNo/mock-pay —— 支付渠道线下未接时模拟支付成功（等价于回调）。
// 仅在 cfg.PaymentEnabled()==false 时放行；生产已接支付则 403。
func (s *Server) mockPayOrder(c *gin.Context) {
	if s.cfg != nil && s.cfg.PaymentEnabled() {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "支付渠道已接入，不支持模拟支付"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if err := commerce.NewService(s.dbConn).MockPay(c.Request.Context(), student.ID, c.Param("orderNo")); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10302, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "paid"}})
}
