package api

import (
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) smsLogin(c *gin.Context) {
	var request struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Phone == "" || request.Code == "" {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil || s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
		return
	}
	student, err := s.identity.LoginBySMS(c.Request.Context(), request.Phone, request.Code, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "验证码错误或过期"})
		return
	}
	session, err := s.identity.CreateSession(c.Request.Context(), student.ID, timeNowUTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
		return
	}
	c.SetCookie("cw_tc_sid", session.Token, int(timeUntil(session.ExpiresAt)), "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"student_no": student.StudentNo, "name": student.Name}})
}

func (s *Server) resetPassword(c *gin.Context) {
	var request struct {
		Phone       string `json:"phone"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Phone == "" || request.Code == "" || request.NewPassword == "" {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil || s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
		return
	}
	if err := s.identity.ResetPasswordBySMS(c.Request.Context(), request.Phone, request.Code, request.NewPassword, time.Now().UTC()); err != nil {
		if err == identity.ErrSMSInvalid {
			c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "验证码错误或过期"})
			return
		}
		c.JSON(http.StatusOK, envelope{Code: 10003, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
}

func (s *Server) rebindPhone(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var request struct {
		OldCode string `json:"old_code"`
		Phone   string `json:"phone"`
		NewCode string `json:"new_code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.OldCode == "" || request.Phone == "" || request.NewCode == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	if s.identity == nil || s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
		return
	}
	if err := s.identity.RebindPhone(c.Request.Context(), student.ID, request.OldCode, request.Phone, request.NewCode, time.Now().UTC()); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
}

func (s *Server) deleteAccount(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	if err := s.identity.AnonymizeAccount(c.Request.Context(), student.ID); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: err.Error()})
		return
	}
	c.SetCookie("cw_tc_sid", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "anonymized"}})
}
