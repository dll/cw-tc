package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// adminCourses GET /admin/courses —— 全部课程（含未上架/待审），按状态过滤可选。
func (s *Server) adminCourses(c *gin.Context) {
	query := `SELECT course_code, source, name, line, status, price_cents, teacher_id, created_at FROM course`
	args := make([]any, 0)
	if st := c.Query("status"); st != "" {
		query += ` WHERE status = $1`
		args = append(args, st)
	}
	query += ` ORDER BY id DESC`
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, source, name, line, status, createdAt string
		var priceCents int64
		var teacherID *int64
		if err := rows.Scan(&code, &source, &name, &line, &status, &priceCents, &teacherID, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"course_code": code, "source": source, "name": name, "line": line,
			"status": status, "price_cents": priceCents, "teacher_id": teacherID, "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
