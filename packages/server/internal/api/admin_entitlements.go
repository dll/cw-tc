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

// adminListEntitlements GET /admin/entitlements —— 权益台账（只回脱敏后的学员标识）。
func (s *Server) adminListEntitlements(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	where := []string{"1=1"}
	args := make([]any, 0, 6)
	addArg := func(value any, clause string) {
		args = append(args, value)
		where = append(where, clause+"$"+strconv.Itoa(len(args)))
	}
	if raw := strings.TrimSpace(c.Query("student_id")); raw != "" {
		studentID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || studentID <= 0 {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "学员编号非法"})
			return
		}
		addArg(studentID, "e.student_id = ")
	}
	if code := strings.TrimSpace(c.Query("course_code")); code != "" {
		addArg(code, "c.course_code = ")
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		if source != "order" && source != "free" && source != "grant" {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "权益来源只接受 order / free / grant"})
			return
		}
		addArg(source, "e.source = ")
	}
	switch status := strings.TrimSpace(c.Query("status")); status {
	case "":
	case "active":
		where = append(where, "e.revoked_at IS NULL")
	case "revoked":
		where = append(where, "e.revoked_at IS NOT NULL")
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 active / revoked"})
		return
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM entitlement e JOIN course c ON c.id = e.course_id`+clause, args...).Scan(&total); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "权益台账读取失败"})
		return
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT e.id, s.student_no, s.name, c.course_code, c.name, e.source,
		        COALESCE(o.order_no,''), e.grant_reason, e.granted_at, e.revoked_at, e.revoke_reason
		 FROM entitlement e
		 JOIN student s ON s.id = e.student_id
		 JOIN course c ON c.id = e.course_id
		 LEFT JOIN orders o ON o.id = e.order_id`+clause+
			` ORDER BY e.granted_at DESC, e.id DESC LIMIT $`+strconv.Itoa(len(listArgs)-1)+
			` OFFSET $`+strconv.Itoa(len(listArgs)), listArgs...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "权益台账读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var studentNo, studentName, courseCode, courseName, source, orderNo, grantedAt string
		var grantReason, revokedAt, revokeReason sql.NullString
		if err := rows.Scan(&id, &studentNo, &studentName, &courseCode, &courseName, &source,
			&orderNo, &grantReason, &grantedAt, &revokedAt, &revokeReason); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "权益台账读取失败"})
			return
		}
		status := "active"
		if revokedAt.Valid {
			status = "revoked"
		}
		items = append(items, gin.H{
			"id": id, "student_no": studentNo, "student_name": studentName,
			"course_code": courseCode, "course_name": courseName, "source": source,
			"order_no": nullIfBlank(orderNo), "grant_reason": nullableString(grantReason),
			"granted_at": grantedAt, "status": status,
			"revoked_at": nullableString(revokedAt), "revoke_reason": nullableString(revokeReason),
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "权益台账读取失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "total": total}})
}

// adminRevokeEntitlementByID DELETE /admin/entitlements/:id —— 撤销授权权益（事由必填 + 审计）。
//
// 只收权益，不撤证书（PRD §7.3 G-7）；退款走退款流程，二者不等价。
func (s *Server) adminRevokeEntitlementByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "权益编号非法"})
		return
	}
	var input struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Reason) == "" || !input.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "撤销权益必须填写事由并二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var studentID, courseID int64
	var revokedAt sql.NullString
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT student_id, course_id, revoked_at FROM entitlement WHERE id = $1`, id).
		Scan(&studentID, &courseID, &revokedAt); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "权益不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "权益读取失败"})
		return
	}
	if revokedAt.Valid {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "权益已撤销"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE entitlement SET revoked_at = $1, revoke_reason = 'admin'
		 WHERE id = $2 AND revoked_at IS NULL`, now, id)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "撤销权益失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "权益已被撤销"})
		return
	}
	// 权益撤销后离线授权同步失效，避免“撤了权益还能离线看”。
	_, _ = s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE download_grant SET status = 'revoked', revoked_reason = 'admin_revoked'
		 WHERE student_id = $1 AND course_id = $2 AND status = 'valid'`, studentID, courseID)

	after, _ := json.Marshal(gin.H{"reason": strings.TrimSpace(input.Reason), "revoke_reason": "admin"})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "entitlement.revoke", TargetType: "entitlement", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"id": id, "status": "revoked", "revoked_at": now, "revoke_reason": "admin",
	}})
}
