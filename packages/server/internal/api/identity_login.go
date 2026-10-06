package api

import (
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

func (s *Server) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	student, err := s.identity.Login(c.Request.Context(), request.Phone, request.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "手机号或密码错误"})
		return
	}
	session, err := s.identity.CreateSession(c.Request.Context(), student.ID, timeNowUTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
		return
	}
	c.SetCookie("cw_tc_sid", session.Token, int(timeUntil(session.ExpiresAt)), "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"student_no": student.StudentNo,
		"name":       student.Name,
		"phone":      student.Phone,
	}})
}

var timeNowUTC = func() time.Time { return time.Now().UTC() }
var timeUntil = func(t time.Time) time.Duration { return time.Until(t) }

var _ = identity.ErrInvalidCredentials
