package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminListUsers GET /admin/users —— 四类用户统一列表（默认脱敏，身份证号永不返回）。
func (s *Server) adminListUsers(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数错误"})
		return
	}
	userType := c.Query("type")
	switch userType {
	case "", "student", "teacher", "admin_super", "admin_ops":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "用户类型错误"})
		return
	}
	const usersSQL = `
		SELECT id, 'student' AS user_type, student_no, CAST(NULL AS TEXT) AS account,
		       name, phone, status, created_at, registration_reviewed_at, registration_review_status FROM student WHERE $1 = '' OR $1 = 'student'
		UNION ALL
		SELECT id, role AS user_type, CAST(NULL AS TEXT) AS student_no, account,
		       name, CAST(NULL AS TEXT) AS phone, status, created_at, NULL AS registration_reviewed_at, CAST(NULL AS TEXT) AS registration_review_status FROM staff
		       WHERE $1 = '' OR role = $1`
	var total int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM (`+usersSQL+`) AS users`, userType).Scan(&total); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, user_type, student_no, account, name, phone, status, created_at, registration_reviewed_at, registration_review_status
		 FROM (`+usersSQL+`) AS users ORDER BY CASE WHEN registration_review_status='pending' THEN 0 ELSE 1 END, created_at DESC, user_type, id DESC LIMIT $2 OFFSET $3`,
		userType, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var userType, name, status, createdAt string
		var studentNo, account, phone sql.NullString
		var reviewed sql.NullString
		var reviewStatus sql.NullString
		if err := rows.Scan(&id, &userType, &studentNo, &account, &name, &phone, &status, &createdAt, &reviewed, &reviewStatus); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		maskedPhone := ""
		if phone.Valid && phone.String != "" {
			maskedPhone = redact.Phone(phone.String)
		}
		items = append(items, gin.H{
			"id": id, "user_type": userType, "student_no": studentNo.String,
			"account": account.String, "name": name, "phone": maskedPhone,
			"status": status, "created_at": createdAt,
			"registration_status": func() string {
				if userType != "student" {
					return "n/a"
				}
				if reviewStatus.Valid {
					return reviewStatus.String
				}
				return "approved"
			}(),
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "total": total}})
}

// adminRevealUser POST /admin/users/:id/reveal —— 看全号（事由必填 + 写审计；宽 ≠ 无痕）。
func (s *Server) adminRevealUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Reason == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "事由必填"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var phone string
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT phone FROM student WHERE id = $1`, id).Scan(&phone); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "用户不存在"})
		return
	}
	// 写审计（每次 reveal 都留痕，铁律 8 v2.6）；事由以 JSON 快照存入 after 列
	reasonJSON, _ := json.Marshal(map[string]string{"reason": req.Reason})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "user.reveal_phone", TargetType: "student", TargetID: strconv.FormatInt(id, 10),
		After: string(reasonJSON), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"phone": phone}})
}
