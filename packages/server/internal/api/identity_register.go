package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/redact"
	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	IDCard   string `json:"id_card"`
	Password string `json:"password"`
	SMSCode  string `json:"sms_code"`
	SMSScene string `json:"sms_scene"`
}

func (s *Server) register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	student, err := s.identity.Register(c.Request.Context(), identity.RegisterInput{
		Name: request.Name, Phone: request.Phone, IDCard: request.IDCard, Password: request.Password,
		SMSCode: request.SMSCode, SMSScene: request.SMSScene,
	})
	if err != nil {
		code := 10002
		if err == identity.ErrPhoneRegistered {
			code = 10103
		} else if err == identity.ErrIDCardRegistered {
			code = 10104
		} else if err == identity.ErrSMSInvalid {
			code = 10105
		}
		c.JSON(http.StatusOK, envelope{Code: code, Msg: "注册失败"})
		return
	}
	// 注册即登录：创建会话，返回 token（移动端存安全存储、走 Bearer）。
	session, err := s.identity.CreateSession(c.Request.Context(), student.ID, timeNowUTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
		return
	}
	c.SetCookie("cw_tc_sid", session.Token, int(timeUntil(session.ExpiresAt)), "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"token":      session.Token,
		"student_no": student.StudentNo,
		"name":       student.Name,
		"phone":      redact.Phone(student.Phone),
	}})
}
