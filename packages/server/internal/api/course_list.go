package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) listPublicCourses(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	query := `SELECT c.course_code, c.name, c.intro, c.price_cents, c.original_price_cents, c.major_id, m.name FROM course c LEFT JOIN major m ON m.id = c.major_id WHERE c.status='published'`
	args := make([]any, 0)
	if majorStr := c.Query("major_id"); majorStr != "" {
		if majorID, err := strconv.ParseInt(majorStr, 10, 64); err == nil {
			query += ` AND c.major_id = $1`
			args = append(args, majorID)
		}
	}
	query += ` ORDER BY c.sort_weight DESC, c.id ASC`
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name string
		var intro *string
		var price int64
		var original *int64
		var majorID *int64
		var majorName *string
		if err := rows.Scan(&code, &name, &intro, &price, &original, &majorID, &majorName); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
			return
		}
		items = append(items, gin.H{"course_code": code, "name": name, "intro": intro, "price_cents": price, "original_price_cents": original, "major_id": majorID, "major_name": majorName})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
