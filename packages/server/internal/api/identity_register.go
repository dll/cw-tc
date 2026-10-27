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
		RequireReview: true,
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
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"student_no": student.StudentNo,
		"name":       student.Name,
		"phone":      redact.Phone(student.Phone),
		"status":     "pending_review",
	}})
}
