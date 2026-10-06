package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/redact"
	"github.com/gin-gonic/gin"
)

const studentContextKey = "student"

func authRequired(service *identity.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		if cookie, err := c.Cookie("cw_tc_sid"); err == nil {
			token = cookie
		}
		if token == "" {
			header := c.GetHeader("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			}
		}
		if service == nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
			return
		}
		student, err := service.Authenticate(c.Request.Context(), token, time.Now().UTC())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
			return
		}
		c.Set(studentContextKey, student)
		c.Next()
	}
}

func (s *Server) me(c *gin.Context) {
	value, exists := c.Get(studentContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	student := value.(identity.Student)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"student_no": student.StudentNo,
		"name":       student.Name,
		"phone":      redact.Phone(student.Phone),
	}})
}
