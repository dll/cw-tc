package api

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/identity"
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
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "参数格式错误"})
		return
	}
	if !consumeCaptcha(request.CaptchaID, request.CaptchaAnswer) {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "图形验证码错误"})
		return
	}
	if identity.ValidatePhone(strings.TrimSpace(request.Phone)) != nil {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "手机号格式错误"})
		return
	}
	if s.sms == nil || s.smsSender == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用（渠道未配置）"})
		return
	}
	now := time.Now().UTC()
	if !s.sms.ClaimDaily(request.Phone, now, 10) {
		c.JSON(http.StatusTooManyRequests, envelope{Code: 10903, Msg: "该手机号今日短信次数已达上限"})
		return
	}
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	code := fmt.Sprintf("%06d", binary.BigEndian.Uint32(raw[:])%1000000)
	if _, err := s.sms.Issue(c.Request.Context(), request.Phone, request.Scene, code, now); err != nil {
		if errors.Is(err, identity.ErrSMSCooldown) {
			c.JSON(http.StatusTooManyRequests, envelope{Code: 10903, Msg: "发送过于频繁，请 60 秒后重试"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信验证码生成失败"})
		return
	}
	if err := s.smsSender.Send(c.Request.Context(), request.Phone, code); err != nil {
		s.sms.Revoke(request.Phone, request.Scene)
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "短信发送失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"expires_in": 600}})
}

var _ = gin.HandlerFunc(nil)
