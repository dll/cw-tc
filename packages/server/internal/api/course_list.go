package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) listPublicCourses(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT course_code, name, intro, price_cents, original_price_cents, major_id FROM course WHERE status='published' ORDER BY sort_weight DESC, id ASC`)
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
		if err := rows.Scan(&code, &name, &intro, &price, &original, &majorID); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
			return
		}
		items = append(items, gin.H{"course_code": code, "name": name, "intro": intro, "price_cents": price, "original_price_cents": original, "major_id": majorID})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
