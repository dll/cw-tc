package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/staff"
)

// teacherCourses GET /teacher/courses —— 教师查看自己的课程（强制带归属条件，铁律 8）。
func (s *Server) teacherCourses(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT course_code, name, line, status, price_cents, major_id, created_at FROM course WHERE teacher_id = $1 ORDER BY id DESC`, st.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, line, status, createdAt string
		var priceCents int64
		var majorID *int64
		if err := rows.Scan(&code, &name, &line, &status, &priceCents, &majorID, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"course_code": code, "name": name, "line": line, "status": status,
			"price_cents": priceCents, "major_id": majorID, "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// teacherCreateCourse POST /teacher/courses —— 教师新建课程草稿（platform 事实源，铁律 1）。
func (s *Server) teacherCreateCourse(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	var req struct {
		Name    string `json:"name"`
		Line    string `json:"line"`
		MajorID *int64 `json:"major_id"`
		Intro   string `json:"intro"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	line := req.Line
	if line == "" {
		line = "main"
	}
	code := newPlatformCourseCode()
	_, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO course(course_code, source, name, intro, line, major_id, status, teacher_id, created_at, updated_at)
		 VALUES($1, 'platform', $2, $3, $4, $5, 'proposed', $6, $7, $8)`,
		code, req.Name, req.Intro, line, req.MajorID, st.ID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10002, Msg: "创建失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": code}})
}

// teacherSubmitCourse POST /teacher/courses/:code/submit —— 提交审核（proposed → submitted，归属校验）。
func (s *Server) teacherSubmitCourse(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	code := c.Param("code")
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course SET status = 'submitted', submitted_at = $1 WHERE course_code = $2 AND teacher_id = $3 AND status IN ('proposed','rejected')`,
		time.Now().UTC(), code, st.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10002, Msg: "课程不存在、非本人课程或状态不可提交"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// newPlatformCourseCode 生成平台课程码（platform 事实源，不得伪装成上游）。
func newPlatformCourseCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "platform-" + hex.EncodeToString(b)
}
