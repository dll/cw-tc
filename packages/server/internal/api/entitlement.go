package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) listEntitlements(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	items, err := commerce.NewService(s.dbConn).ListEntitlements(c.Request.Context(), student.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 10902, Msg: "权益查询失败"})
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, gin.H{"course_id": item.CourseID, "order_id": item.OrderID, "source": item.Source, "status": item.Status, "granted_at": item.GrantedAt})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": data}})
}
