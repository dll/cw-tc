package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

// listMajors 返回启用的专业分类（只读公开数据）。
func (s *Server) listMajors(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "专业数据不可用"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT id, name FROM major WHERE status = 'enabled' ORDER BY sort ASC, id ASC`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "专业数据不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "专业数据不可用"})
			return
		}
		items = append(items, gin.H{"id": id, "name": name})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "专业数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// listCourseLectures 返回已上架课程的讲次列表（含试学标记）。
func (s *Server) listCourseLectures(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT id FROM course WHERE course_code = $1 AND status = 'published'`, c.Param("courseCode")).Scan(&courseID); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT no, title, is_preview FROM lecture WHERE course_id = $1 ORDER BY no ASC`, courseID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var no int
		var title string
		var isPreview bool
		if err := rows.Scan(&no, &title, &isPreview); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
			return
		}
		items = append(items, gin.H{"no": no, "title": title, "is_preview": isPreview})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
