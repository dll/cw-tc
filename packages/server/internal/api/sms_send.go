package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type smsSendRequest struct {
	Phone         string `json:"phone"`
	Scene         string `json:"scene"`
	CaptchaID     string `json:"captcha_id"`
	CaptchaAnswer string `json:"captcha_answer"`
}

func (s *Server) sendSMS(c *gin.Context) {
	var request smsSendRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Phone == "" || request.Scene == "" || request.CaptchaID == "" || request.CaptchaAnswer == "" {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	// 批次 1 先固定 fail-closed：验证码前置校验完成后，未配置短信通道不得尝试发送。
	c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
}

var _ = gin.HandlerFunc(nil)
