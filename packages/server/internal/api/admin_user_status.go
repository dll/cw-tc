package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

var statusReasonKinds = map[string]bool{
	"sms_abuse":         true,
	"payment_abuse":     true,
	"identity_mismatch": true,
	"exam_fraud":        true,
	"serious_violation": true,
	"login_fail":        true,
}

// adminUpdateUserStatus PATCH /admin/users/:id/status —— 停用 / 封禁 / 解封（PRD §7.10.10（三））。
func (s *Server) adminUpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "用户编号非法"})
		return
	}
	var input struct {
		Status     string `json:"status"`
		ReasonKind string `json:"reason_kind"`
		Note       string `json:"note"`
		Until      string `json:"until"`
		Confirm    bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "高危操作必须二次确认"})
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if len([]rune(input.Note)) > 500 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "说明过长（≤500 字）"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)

	var currentStatus string
	var reasonKind, note, until sql.NullString
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT status, status_reason_kind, status_note, status_until FROM student WHERE id = $1`, id).
		Scan(&currentStatus, &reasonKind, &note, &until); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "用户不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "用户状态读取失败"})
		return
	}
	if currentStatus == "anonymized" || currentStatus == "deleting" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "注销中的账号不能修改状态"})
		return
	}
	if currentStatus == input.Status {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "账号已是该状态"})
		return
	}

	var untilValue, reasonValue any
	switch input.Status {
	case "active":
		// 解封：清空原因与期限；不恢复已撤销证书 / 已退款订单。
	case "suspended":
		when, ok := parseFutureTime(input.Until)
		if !ok {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "停用必须填写未来期限"})
			return
		}
		untilValue = when.Format(time.RFC3339)
		if !statusReasonKinds[input.ReasonKind] || input.Note == "" {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "停用必须填写原因类别与说明"})
			return
		}
		reasonValue = input.ReasonKind
	case "banned":
		if !statusReasonKinds[input.ReasonKind] || input.Note == "" {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "封禁必须填写原因类别与说明"})
			return
		}
		reasonValue = input.ReasonKind
		if strings.TrimSpace(input.Until) == "" {
			// 永久封禁仅 super。
			if admin.Role != staff.RoleAdminSuper {
				c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "永久封禁仅超级管理员可执行"})
				return
			}
		} else {
			when, ok := parseFutureTime(input.Until)
			if !ok {
				c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "封禁期限必须是未来时间"})
				return
			}
			untilValue = when.Format(time.RFC3339)
		}
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 active / suspended / banned"})
		return
	}

	before, _ := json.Marshal(gin.H{
		"status": currentStatus, "reason_kind": nullableString(reasonKind),
		"note": nullableString(note), "until": nullableString(until),
	})
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE student SET status = $1, status_reason_kind = $2, status_note = $3,
		        status_until = $4, status_by = $5, updated_at = $6
		 WHERE id = $7`,
		input.Status, reasonValue, nullIfBlank(input.Note), untilValue, admin.ID,
		time.Now().UTC().Format(time.RFC3339), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "用户状态更新失败"})
		return
	}
	// 停用 / 封禁必须立即踢出全部会话，否则旧 token 仍可继续使用。
	if input.Status != "active" && s.identity != nil {
		_ = s.identity.RevokeAllSessions(c.Request.Context(), id)
	}
	after, _ := json.Marshal(gin.H{
		"status": input.Status, "reason_kind": nullIfBlank(input.ReasonKind),
		"note": nullIfBlank(input.Note), "until": untilValue,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "user.status_change", TargetType: "student", TargetID: strconv.FormatInt(id, 10),
		Before: string(before), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"id": id, "status": input.Status, "reason_kind": nullIfBlank(input.ReasonKind),
		"note": nullIfBlank(input.Note), "until": untilValue,
	}})
}

func parseFutureTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil || !parsed.After(time.Now().UTC()) {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func nullIfBlank(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
