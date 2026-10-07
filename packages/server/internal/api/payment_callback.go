package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/gin-gonic/gin"
)

func (s *Server) paymentCallback(c *gin.Context) {
	var input commerce.CallbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	if s.dbConn == nil || s.cfg == nil || s.cfg.WXPayAPIv3Key == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "支付通道未配置"})
		return
	}
	if err := commerce.NewPaymentService(s.dbConn, []byte(s.cfg.WXPayAPIv3Key)).HandleCallback(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10304, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "paid"}})
}
