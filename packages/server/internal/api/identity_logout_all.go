package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) logoutAll(c *gin.Context) {
	token, err := c.Cookie("cw_tc_sid")
	if err != nil || s.identity == nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	studentID, err := s.identity.Authenticate(c.Request.Context(), token, timeNowUTC())
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	if err := s.identity.RevokeAllSessions(c.Request.Context(), studentID.ID); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
		return
	}
	c.SetCookie("cw_tc_sid", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"logged_out": true}})
}
