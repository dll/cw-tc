package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) listMySuggestions(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, course_code, body, status, reply, created_at, updated_at
		 FROM suggestion WHERE student_id=$1 ORDER BY created_at DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "建议查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var body, status, createdAt, updatedAt string
		var courseCode, reply sql.NullString
		if err := rows.Scan(&id, &courseCode, &body, &status, &reply, &createdAt, &updatedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "建议查询失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "course_code": nullableString(courseCode), "body": body,
			"status": status, "reply": nullableString(reply),
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) createSuggestion(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		CourseCode string `json:"course_code"`
		Body       string `json:"body"`
	}
	body := strings.TrimSpace(input.Body)
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "建议参数无效"})
		return
	}
	body = strings.TrimSpace(input.Body)
	if len([]rune(body)) < 10 || len([]rune(body)) > 1000 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "建议内容需 10-1000 字"})
		return
	}
	var course any
	if strings.TrimSpace(input.CourseCode) != "" {
		var n int
		if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM course WHERE course_code=$1 AND deleted_at IS NULL`, input.CourseCode).Scan(&n); err != nil || n == 0 {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
			return
		}
		course = input.CourseCode
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO suggestion(student_id,course_code,body,status,created_at,updated_at) VALUES($1,$2,$3,'pending',$4,$4)`,
		student.ID, course, body, now); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "建议保存失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
