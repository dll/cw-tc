package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) changePassword(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var request struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.OldPassword == "" || request.NewPassword == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	err := s.identity.ChangePassword(c.Request.Context(), student.ID, request.OldPassword, request.NewPassword)
	if err == identity.ErrInvalidCredentials {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "旧密码错误"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
}
