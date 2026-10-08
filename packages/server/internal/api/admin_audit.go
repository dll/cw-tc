package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// adminAudit GET /admin/audit —— 审计日志（只读，只增不改不删）。
func (s *Server) adminAudit(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	query := `SELECT id, actor_type, actor_id, actor_role, action, target_type, target_id, created_at FROM audit_log`
	args := make([]any, 0)
	if action := c.Query("action"); action != "" {
		query += ` WHERE action = $1`
		args = append(args, action)
	}
	n := len(args) + 1
	query += fmt.Sprintf(` ORDER BY id DESC LIMIT $%d OFFSET $%d`, n, n+1)
	args = append(args, limit, offset)
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, actorID int64
		var actorType, actorRole, action, targetType, targetID, createdAt string
		if err := rows.Scan(&id, &actorType, &actorID, &actorRole, &action, &targetType, &targetID, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"id": id, "actor_type": actorType, "actor_id": actorID, "actor_role": actorRole,
			"action": action, "target_type": targetType, "target_id": targetID, "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
