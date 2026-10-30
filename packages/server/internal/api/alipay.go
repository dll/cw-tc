package api

import (
	"net/http"
	"strings"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) alipayClient() commerce.AlipayClient {
	return commerce.AlipayClient{AppID: s.cfg.AlipayAppID, PrivateKeyPath: s.cfg.AlipayPrivKey, PublicKeyPath: s.cfg.AlipayPubKey, NotifyURL: s.cfg.AlipayNotify, Gateway: s.cfg.AlipayGateway}
}

func (s *Server) alipayPrecreate(c *gin.Context) {
	if s.cfg == nil || !s.cfg.AlipayEnabled() {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10306, Msg: "支付宝尚未配置"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	order, err := commerce.NewService(s.dbConn).GetOrder(c.Request.Context(), c.Param("orderNo"))
	if err != nil || order.StudentID != student.ID {
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "订单不存在"})
		return
	}
	if _, err = commerce.NewService(s.dbConn).PreparePayment(c.Request.Context(), student.ID, commerce.PaymentParams{OrderNo: order.OrderNo, Channel: "alipay", AmountCents: order.AmountCents}); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10302, Msg: "订单不可支付"})
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
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"order_no": order.OrderNo, "qr_code": result.QRCode, "amount_cents": order.AmountCents, "expires_at": order.ExpiredAt}})
}

func (s *Server) alipayNotify(c *gin.Context) {
	if s.cfg == nil || !s.cfg.AlipayEnabled() {
		c.String(http.StatusServiceUnavailable, "failure")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	form := c.Request.PostForm
	if err := s.alipayClient().VerifyNotify(form); err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	if form.Get("trade_status") != "TRADE_SUCCESS" && form.Get("trade_status") != "TRADE_FINISHED" {
		c.String(http.StatusOK, "success")
		return
	}
	amount, err := commerce.AlipayAmountCents(form.Get("total_amount"))
	if err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	tradeNo := strings.TrimSpace(form.Get("trade_no"))
	orderNo := strings.TrimSpace(form.Get("out_trade_no"))
	if tradeNo == "" || orderNo == "" {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	err = commerce.NewPaymentService(s.dbConn, nil).HandleVerifiedCallback(c.Request.Context(), commerce.CallbackInput{OrderNo: orderNo, Channel: "alipay", ChannelTxnID: tradeNo, IdempotencyKey: "alipay:" + tradeNo, AmountCents: amount})
	if err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	s.notifyOrderPaid(c.Request.Context(), orderNo)
	c.String(http.StatusOK, "success")
}
