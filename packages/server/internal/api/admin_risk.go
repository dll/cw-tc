package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/risk"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminRiskOverview GET /admin/risk —— 风控规则与黑名单（手机号脱敏）。
func (s *Server) adminRiskOverview(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	switch status {
	case "", "active", "expired", "appealed":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 active / expired / appealed"})
		return
	}
	targetType := strings.TrimSpace(c.Query("target_type"))
	switch targetType {
	case "", "phone", "device", "payment_account":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "对象类型只接受 phone / device / payment_account"})
		return
	}
	where := ` WHERE ($1 = '' OR status = $1) AND ($2 = '' OR target_type = $2)`
	var total int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM risk_blacklist`+where, status, targetType).Scan(&total); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "风控数据读取失败"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, target_type, target_value, reason, status, expires_at, created_at,
		        COALESCE(appeal_reason,''), appealed_at
		 FROM risk_blacklist`+where+` ORDER BY id DESC LIMIT $3 OFFSET $4`,
		status, targetType, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "风控数据读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var itemType, itemValue, reason, itemStatus, expiresAt, createdAt, appealReason string
		var appealedAt sql.NullString
		if err := rows.Scan(&id, &itemType, &itemValue, &reason, &itemStatus, &expiresAt,
			&createdAt, &appealReason, &appealedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "风控数据读取失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "target_type": itemType, "target_value_masked": maskBlacklistValue(itemType, itemValue),
			"reason": reason, "status": itemStatus, "expires_at": expiresAt, "created_at": createdAt,
			"appeal_reason": appealReason, "appealed_at": nullableString(appealedAt),
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "风控数据读取失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items": items, "total": total,
		"rules": gin.H{
			"refund_abuse_limit":       risk.DefaultRefundAbuseLimit,
			"refund_abuse_window_days": risk.DefaultRefundAbuseWindowDays,
			"note":                     "黑名单必须有期限且可申诉；多账号关联只提示、不硬拦",
		},
	}})
}

// adminAddBlacklist POST /admin/risk/blacklist —— 加入黑名单（原因 + 期限必填 + 二次确认 + 审计）。
func (s *Server) adminAddBlacklist(c *gin.Context) {
	var input struct {
		TargetType  string `json:"target_type"`
		TargetValue string `json:"target_value"`
		Reason      string `json:"reason"`
		ExpiresAt   string `json:"expires_at"`
		Confirm     bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm ||
		strings.TrimSpace(input.TargetValue) == "" || strings.TrimSpace(input.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "加入黑名单必须填写对象、原因、期限并二次确认"})
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ExpiresAt))
	if err != nil || !expiresAt.After(time.Now().UTC()) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "黑名单期限必须是未来时间"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	targetType := risk.TargetType(strings.TrimSpace(input.TargetType))
	if err := risk.AddBlacklist(c.Request.Context(), s.dbConn, targetType,
		strings.TrimSpace(input.TargetValue), strings.TrimSpace(input.Reason), expiresAt, admin.ID); err != nil {
		switch err {
		case risk.ErrBlacklistTarget, risk.ErrBlacklistInput, risk.ErrBlacklistEmpty:
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
		default:
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "黑名单写入失败"})
		}
		return
	}
	after, _ := json.Marshal(gin.H{
		"target_type": string(targetType), "target_value": maskBlacklistValue(string(targetType), input.TargetValue),
		"reason": strings.TrimSpace(input.Reason), "expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "risk.blacklist_add", TargetType: "risk_blacklist", TargetID: string(targetType),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminResolveBlacklistAppeal PATCH /admin/risk/blacklist/:id —— 处理黑名单申诉。
func (s *Server) adminResolveBlacklistAppeal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "黑名单编号非法"})
		return
	}
	var input struct {
		Accept  bool   `json:"accept"`
		Note    string `json:"note"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.Note) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "处理申诉必须填写结论并二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if err := risk.ResolveAppeal(c.Request.Context(), s.dbConn, id, input.Accept); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "该条黑名单未处于申诉中"})
		return
	}
	after, _ := json.Marshal(gin.H{"accept": input.Accept, "note": strings.TrimSpace(input.Note)})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "risk.blacklist_appeal", TargetType: "risk_blacklist", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func maskBlacklistValue(targetType, value string) string {
	if targetType == "phone" {
		return redact.Phone(value)
	}
	if len(value) <= 8 {
		return redact.Mask
	}
	return value[:4] + redact.Mask + value[len(value)-4:]
}
