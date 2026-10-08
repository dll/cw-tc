package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminListUsers GET /admin/users —— 学员列表（默认脱敏，身份证号永不返回）。
func (s *Server) adminListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, student_no, name, phone, status, created_at FROM student ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var studentNo, name, phone, status, createdAt string
		if err := rows.Scan(&id, &studentNo, &name, &phone, &status, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"id": id, "student_no": studentNo, "name": name,
			"phone": redact.Phone(phone), "status": status, "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
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
