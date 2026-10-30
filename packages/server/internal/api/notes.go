package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) listNotes(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	query := `SELECT n.id, co.course_code, n.lecture_no, n.title, n.content_md, n.source, n.created_at, n.updated_at FROM note n JOIN course co ON co.id=n.course_id WHERE n.student_id=$1`
	args := []any{student.ID}
	if code := strings.TrimSpace(c.Query("course_code")); code != "" {
		args = append(args, code)
		query += ` AND co.course_code=$2`
	}
	query += ` ORDER BY n.updated_at DESC, n.id DESC LIMIT 200`
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "笔记查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var code, title, body, source, createdAt, updatedAt string
		var lectureNo sql.NullInt64
		if err := rows.Scan(&id, &code, &lectureNo, &title, &body, &source, &createdAt, &updatedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "笔记查询失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "course_code": code, "lecture_no": nullableInt(lectureNo),
			"title": title, "body": body, "source": source,
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) createNote(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		CourseCode string `json:"course_code"`
		LectureNo  *int   `json:"lecture_no"`
		Title      string `json:"title"`
		Body       string `json:"body"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.CourseCode == "" || strings.TrimSpace(input.Body) == "" || len([]rune(input.Body)) > 5000 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "笔记参数无效"})
		return
	}
	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT id FROM course WHERE course_code=$1 AND deleted_at IS NULL`, input.CourseCode).Scan(&courseID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var id int64
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO note(student_id,course_id,lecture_no,title,content_md,source,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'manual',$6,$6) RETURNING id`,
		student.ID, courseID, input.LectureNo, strings.TrimSpace(input.Title), strings.TrimSpace(input.Body), now).Scan(&id)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "笔记保存失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

func (s *Server) updateNote(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Body) == "" || len([]rune(input.Body)) > 5000 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "笔记参数无效"})
		return
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE note SET title=$1, content_md=$2, updated_at=$3 WHERE id=$4 AND student_id=$5`,
		strings.TrimSpace(input.Title), strings.TrimSpace(input.Body), time.Now().UTC().Format(time.RFC3339), c.Param("id"), student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "笔记保存失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "笔记不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) deleteNote(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	_, err := s.dbConn.ExecContext(c.Request.Context(), `DELETE FROM note WHERE id=$1 AND student_id=$2`, c.Param("id"), student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "笔记删除失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
