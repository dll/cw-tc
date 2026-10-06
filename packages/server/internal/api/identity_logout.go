package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) logout(c *gin.Context) {
	token, err := c.Cookie("cw_tc_sid")
	if err == nil && s.identity != nil {
		if revokeErr := s.identity.RevokeSession(c.Request.Context(), token); revokeErr != nil && revokeErr != identity.ErrSessionNotFound {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
			return
		}
	}
	c.SetCookie("cw_tc_sid", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"logged_out": true}})
}
