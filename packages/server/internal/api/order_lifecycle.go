package api

import (
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) getOrder(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	order, err := commerce.NewService(s.dbConn).GetOrder(c.Request.Context(), c.Param("orderNo"))
	if err != nil || order.StudentID != student.ID {
		c.JSON(http.StatusNotFound, envelope{Code: 10301, Msg: "订单不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"order_no": order.OrderNo, "status": order.Status, "course_name": order.CourseName, "amount_cents": order.AmountCents, "expired_at": order.ExpiredAt.Format(time.RFC3339)}})
}

func (s *Server) preparePayment(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		Channel     string `json:"channel"`
		AmountCents int64  `json:"amount_cents"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	order, err := commerce.NewService(s.dbConn).PreparePayment(c.Request.Context(), student.ID, commerce.PaymentParams{OrderNo: c.Param("orderNo"), Channel: input.Channel, AmountCents: input.AmountCents})
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10304, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"order_no": order.OrderNo, "channel": input.Channel, "amount_cents": order.AmountCents, "provider": "mock-not-configured"}})
}

func (s *Server) cancelOrder(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	if err := commerce.NewService(s.dbConn).CancelOrder(c.Request.Context(), student.ID, c.Param("orderNo")); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10302, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "closed"}})
}
