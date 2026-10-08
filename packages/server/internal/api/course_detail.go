package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) publicCourseDetail(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	var code, name string
	var intro sql.NullString
	var price, original sql.NullInt64
	var majorID sql.NullInt64
	err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT course_code, name, intro, price_cents, original_price_cents, major_id FROM course WHERE course_code=$1 AND status='published'`, c.Param("courseCode")).Scan(&code, &name, &intro, &price, &original, &majorID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": code, "name": name, "intro": nullableString(intro), "price_cents": price.Int64, "original_price_cents": nullableInt(original), "major_id": nullableInt(majorID)}})
}

func nullableString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}
func nullableInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}
