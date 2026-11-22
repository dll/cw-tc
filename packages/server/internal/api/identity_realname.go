package api

import (
	"errors"
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

type completeIdentityRequest struct {
	IDCard string `json:"id_card"`
}

// completeIdentity POST /me/identity —— 学员补充实名（仅未实名时设置）。
func (s *Server) completeIdentity(c *gin.Context) {
	var request completeIdentityRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	err := s.identity.CompleteIdentity(c.Request.Context(), student.ID, request.IDCard)
	if err != nil {
		code := 10002
		message := "身份证号格式或校验位非法"
		switch {
		case errors.Is(err, identity.ErrIDCardRegistered):
			code = 10104
			message = identity.ErrIDCardRegistered.Error()
		case errors.Is(err, identity.ErrIdentityAlreadySet):
			message = identity.ErrIdentityAlreadySet.Error()
		case errors.Is(err, identity.ErrDatabaseNotConfigured):
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
			return
		}
		c.JSON(http.StatusOK, envelope{Code: code, Msg: message})
		return
	}
	masked, _ := s.identity.MaskedIDCard(c.Request.Context(), student.ID)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"identity_verified": masked != "",
		"identity_masked":   masked,
	}})
}
