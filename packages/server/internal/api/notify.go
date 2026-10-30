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
	unread, _ := notify.UnreadCount(c.Request.Context(), s.dbConn, student.ID)
	prefs, _ := notify.GetPrefs(c.Request.Context(), s.dbConn, student.ID)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items": items, "unread": unread, "prefs": prefs,
	}})
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

// markAllNotificationsRead POST /me/messages/read（兼容 /me/notifications/read）。
func (s *Server) markAllNotificationsRead(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	n, err := notify.MarkAllRead(c.Request.Context(), s.dbConn, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"updated": n}})
}

// getNotificationPrefs GET /me/messages/prefs —— 消息偏好。
func (s *Server) getNotificationPrefs(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	prefs, err := notify.GetPrefs(c.Request.Context(), s.dbConn, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: prefs})
}

// updateNotificationPrefs PATCH /me/messages/prefs —— 广播退订（系统 / 事务不可退订）。
func (s *Server) updateNotificationPrefs(c *gin.Context) {
	var input struct {
		BroadcastOptOut *bool `json:"broadcast_opt_out"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.BroadcastOptOut == nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	prefs := notify.NotificationPrefs{BroadcastOptOut: *input.BroadcastOptOut}
	if err := notify.SetPrefs(c.Request.Context(), s.dbConn, student.ID, prefs); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "通知服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: prefs})
}
