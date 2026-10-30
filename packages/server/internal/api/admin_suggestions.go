package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func (s *Server) adminListSuggestions(c *gin.Context) {
	status := c.DefaultQuery("status", "")
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, student_id, course_code, body, status, reply, created_at FROM suggestion
		 WHERE ($1='' OR status=$1) ORDER BY created_at DESC LIMIT 200`, status)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "建议查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, studentID int64
		var body, statusValue, createdAt string
		var courseCode, reply *string
		if err := rows.Scan(&id, &studentID, &courseCode, &body, &statusValue, &reply, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "建议查询失败"})
			return
		}
		items = append(items, gin.H{"id": id, "student_id": studentID, "course_code": courseCode,
			"body": body, "status": statusValue, "reply": reply, "created_at": createdAt})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminReplySuggestion(c *gin.Context) {
	var input struct {
		Status string `json:"status"`
		Reply  string `json:"reply"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || (input.Status != "accepted" && input.Status != "rejected" && input.Status != "planned") || strings.TrimSpace(input.Reply) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "请填写处理状态与回复"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE suggestion SET status=$1, reply=$2, updated_at=$3 WHERE id=$4`,
		input.Status, strings.TrimSpace(input.Reply), time.Now().UTC().Format(time.RFC3339), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "建议处理失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "建议不存在"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{ActorType: "staff", ActorID: admin.ID,
		ActorRole: string(admin.Role), Action: "suggestion.reply", TargetType: "suggestion",
		TargetID: c.Param("id"), After: input.Status + ":" + input.Reply, IP: c.ClientIP()})
	c.JSON(http.StatusOK, envelope{Code: 0})
}
