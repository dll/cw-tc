package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminResetStudentPassword POST /admin/users/:id/reset-password
//
// 管理员只生成一次性重置码（30 分钟），永远看不到学员密码；重置动作写审计。
func (s *Server) adminResetStudentPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm || strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "事由必填且需二次确认"})
		return
	}
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	now := time.Now().UTC()
	code, expires, last4, err := s.identity.CreateAdminResetCode(c.Request.Context(), id, admin.ID, now)
	if err == identity.ErrInvalidCredentials {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "学员不存在"})
		return
	}
	if err == identity.ErrResetNotAllowed {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "该学员当前状态不允许重置密码"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "重置失败，请稍后重试"})
		return
	}
	after, _ := json.Marshal(map[string]string{
		"reason":     req.Reason,
		"expires_at": expires.Format(time.RFC3339),
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "user.reset_password", TargetType: "student", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"reset_code":  code,
		"expires_at":  expires.Format(time.RFC3339),
		"phone_last4": last4,
		"message":     "请将重置码转交学员，由学员自行设置新密码；管理员不会获知新密码",
	}})
}

// resetPasswordByAdminCode POST /auth/password/reset
//
// 学员用管理员下发的重置码设置新密码（不依赖短信服务）。
func (s *Server) resetPasswordByAdminCode(c *gin.Context) {
	var req struct {
		Phone       string `json:"phone"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Phone == "" || req.Code == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数格式错误"})
		return
	}
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	err := s.identity.ResetPasswordByAdminCode(c.Request.Context(), req.Phone, req.Code, req.NewPassword, time.Now().UTC())
	switch {
	case err == nil:
		c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
	case err == identity.ErrResetCodeInvalid || err == identity.ErrInvalidCredentials:
		// 统一话术，不暴露手机号是否注册。
		c.JSON(http.StatusUnauthorized, envelope{Code: 10105, Msg: "重置码无效或已过期"})
	case err == identity.ErrInvalidInput:
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: "手机号或重置码格式不正确"})
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: err.Error()})
	}
}
