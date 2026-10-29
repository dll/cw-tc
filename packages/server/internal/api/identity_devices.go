package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// sessionToken 读取当前请求的会话令牌（Cookie 优先，其次 Bearer）。
func sessionToken(c *gin.Context) string {
	if cookie, err := c.Cookie("cw_tc_sid"); err == nil && cookie != "" {
		return cookie
	}
	return bearerToken(c)
}

func deviceLabel(userAgent string) string {
	ua := strings.ToLower(userAgent)
	device := ""
	switch {
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		device = "iOS"
	case strings.Contains(ua, "android"):
		device = "Android"
	case strings.Contains(ua, "windows"):
		device = "Windows"
	case strings.Contains(ua, "mac os"), strings.Contains(ua, "macintosh"):
		device = "macOS"
	case strings.Contains(ua, "linux"):
		device = "Linux"
	}
	browser := ""
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "chrome/"), strings.Contains(ua, "crios/"):
		browser = "Chrome"
	case strings.Contains(ua, "firefox/"), strings.Contains(ua, "fxios/"):
		browser = "Firefox"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome/"):
		browser = "Safari"
	}
	switch {
	case device != "" && browser != "":
		return device + " · " + browser
	case device != "":
		return device
	case browser != "":
		return browser
	default:
		return "未知设备"
	}
}

// listDevices GET /me/devices —— 登录设备列表（不返回 token）。
func (s *Server) listDevices(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	items, err := s.identity.ListSessions(c.Request.Context(), student.ID, sessionToken(c), time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "设备列表暂不可用"})
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, gin.H{
			"id":         item.ID,
			"device":     deviceLabel(item.UserAgent),
			"user_agent": item.UserAgent,
			"ip":         item.IP,
			"issued_at":  item.IssuedAt.Format(time.RFC3339),
			"expires_at": item.ExpiresAt.Format(time.RFC3339),
			"current":    item.Current,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": data}})
}

// revokeDevice POST /me/devices/:id/revoke —— 踢出指定设备（只能踢自己的）。
func (s *Server) revokeDevice(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if err := s.identity.RevokeSessionByID(c.Request.Context(), student.ID, id); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "设备不存在或已退出"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"revoked": true}})
}

// logoutAllStudent POST /me/logout-all —— 退出全部设备（Bearer 也生效）。
func (s *Server) logoutAllStudent(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	if err := s.identity.RevokeAllSessions(c.Request.Context(), student.ID); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
		return
	}
	c.SetCookie("cw_tc_sid", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"logged_out": true}})
}
