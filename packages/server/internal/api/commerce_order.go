package api

import (
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) createOrder(c *gin.Context) {
	studentValue, exists := c.Get(studentContextKey)
	if !exists || s.dbConn == nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	student := studentValue.(identity.Student)
	var input struct {
		CourseID int64 `json:"course_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.CourseID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	order, err := commerce.NewService(s.dbConn).CreateOrderFromCourse(c.Request.Context(), student.ID, input.CourseID, c.GetHeader("Idempotency-Key"), time.Now().UTC().Add(30*time.Minute))
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"order_no": order.OrderNo, "status": order.Status, "amount_cents": order.AmountCents}})
}
