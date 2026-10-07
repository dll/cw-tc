package api

import (
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/notify"
	"github.com/gin-gonic/gin"
)

// listNotifications GET /me/notifications —— 学员通知（支持只看未读）。
func (s *Server) listNotifications(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	unreadOnly := c.Query("unread") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := notify.ListNotifications(c.Request.Context(), s.dbConn, student.ID, unreadOnly, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// markNotificationRead POST /me/notifications/:id/read —— 标记已读（只能标自己的）。
func (s *Server) markNotificationRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if err := notify.MarkRead(c.Request.Context(), s.dbConn, id, student.ID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "通知不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
