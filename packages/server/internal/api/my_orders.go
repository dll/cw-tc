package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// listMyOrders GET /me/orders（兼容 GET /orders）—— 我的订单列表（PRD §7.13 L-2）。
func (s *Server) listMyOrders(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10301, Msg: "订单服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	items, err := commerce.NewService(s.dbConn).ListStudentOrders(c.Request.Context(), student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10301, Msg: "订单查询失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// getMyOrder GET /me/orders/:orderNo —— 订单详情（前端轮询 / 详情使用）。
func (s *Server) getMyOrder(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	item, err := commerce.NewService(s.dbConn).GetStudentOrder(c.Request.Context(), student.ID, c.Param("orderNo"))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "订单不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: item})
}

// continueMyOrderPayment POST /me/orders/:orderNo/pay —— 同一订单继续支付（不新建）。
func (s *Server) continueMyOrderPayment(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	service := commerce.NewService(s.dbConn)
	order, err := service.GetOrder(c.Request.Context(), c.Param("orderNo"))
	if err != nil || order.StudentID != student.ID {
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "订单不存在"})
		return
	}
	if _, err = service.PreparePayment(c.Request.Context(), student.ID, commerce.PaymentParams{
		OrderNo: order.OrderNo, Channel: "alipay", AmountCents: order.AmountCents,
	}); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10302, Msg: "订单不可支付"})
		return
	}
	if s.cfg == nil || !s.cfg.AlipayEnabled() {
		// 支付渠道未配置（开发 / staging）：返回 mock，客户端显式点「模拟支付」。
		c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
			"order_no": order.OrderNo, "channel": "mock", "amount_cents": order.AmountCents,
			"expires_at": order.ExpiredAt,
		}})
		return
	}
	result, err := s.alipayClient().Precreate(c.Request.Context(), order.OrderNo, order.CourseName, order.AmountCents)
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10306, Msg: "支付宝下单失败"})
		return
	}
	if _, err = s.dbConn.ExecContext(c.Request.Context(), `UPDATE orders SET channel='alipay' WHERE id=$1 AND status='pending' AND (channel IS NULL OR channel='alipay')`, order.ID); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10302, Msg: "订单状态已变化"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"order_no": order.OrderNo, "channel": "alipay", "qr_code": result.QRCode,
		"amount_cents": order.AmountCents, "expires_at": order.ExpiredAt,
	}})
}

// refundMyOrder POST /me/orders/:orderNo/refund（兼容 /orders/:orderNo/refund）。
func (s *Server) refundMyOrder(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&input)
	ref, err := commerce.NewService(s.dbConn).StudentRefund(c.Request.Context(), student.ID, c.Param("orderNo"), strings.TrimSpace(input.Reason))
	switch {
	case errors.Is(err, commerce.ErrOrderNotFound):
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "订单不存在"})
	case errors.Is(err, commerce.ErrOrderNotRefundable):
		c.JSON(http.StatusConflict, envelope{Code: 10302, Msg: "订单状态不允许退款"})
	case errors.Is(err, commerce.ErrRefundWindowExpired):
		c.JSON(http.StatusConflict, envelope{Code: 10305, Msg: "已超过自助退款窗口，请联系客服"})
	case errors.Is(err, commerce.ErrRefundCertificateIssued):
		c.JSON(http.StatusConflict, envelope{Code: 10305, Msg: "已发证，退款需联系客服"})
	case errors.Is(err, commerce.ErrRefundNeedsManualReview):
		c.JSON(http.StatusConflict, envelope{Code: 10305, Msg: "自助退款次数已达上限，请联系客服"})
	case err != nil:
		c.JSON(http.StatusConflict, envelope{Code: 10306, Msg: "退款处理失败，请联系客服"})
	default:
		s.notifyOrderRefunded(c.Request.Context(), c.Param("orderNo"))
		c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
			"refund_no": ref.RefundNo, "amount_cents": ref.AmountCents, "status": ref.Status,
		}})
	}
}

// myOrderRefundStatus GET /me/orders/:orderNo/refund —— 退款进度（含失败原因）。
func (s *Server) myOrderRefundStatus(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	refund, err := commerce.NewService(s.dbConn).LatestRefund(c.Request.Context(), student.ID, c.Param("orderNo"))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "订单不存在"})
		return
	}
	if refund == nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "暂无退款记录"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: refund})
}
