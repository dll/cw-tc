package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/staff"
)

// teacherCourses GET /teacher/courses —— 教师查看自己的课程（强制带归属条件，铁律 8）。
func (s *Server) teacherCourses(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT course_code, name, line, status, price_cents, major_id, created_at, intro FROM course WHERE teacher_id = $1 ORDER BY id DESC`, st.ID)
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
		var intro *string
		if err := rows.Scan(&code, &name, &line, &status, &priceCents, &majorID, &createdAt, &intro); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"course_code": code, "name": name, "line": line, "status": status,
			"price_cents": priceCents, "major_id": majorID, "created_at": createdAt, "intro": intro,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// teacherCreateCourse POST /teacher/courses —— 教师新建课程草稿（platform 事实源，铁律 1）。
// 课程码由教师填写（不再随机生成），格式：字母/数字/连字符/下划线，2~32 位，唯一。
func (s *Server) teacherCreateCourse(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	var req struct {
		CourseCode string `json:"course_code"`
		Name       string `json:"name"`
		Line       string `json:"line"`
		MajorID    *int64 `json:"major_id"`
		Intro      string `json:"intro"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || strings.TrimSpace(req.CourseCode) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "课程名与课程码必填"})
		return
	}
	code := strings.TrimSpace(req.CourseCode)
	if !validCourseCode(code) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "课程码格式非法（2~32 位字母/数字/连字符/下划线）"})
		return
	}
	line := req.Line
	if line == "" {
		line = "main"
	}
	_, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO course(course_code, source, name, intro, line, major_id, status, teacher_id, created_at, updated_at)
		 VALUES($1, 'platform', $2, $3, $4, $5, 'proposed', $6, $7, $8)`,
		code, req.Name, req.Intro, line, req.MajorID, st.ID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10002, Msg: "课程码已存在或创建失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": code}})
}

// teacherUpdateCourse PUT /teacher/courses/:code —— 修改课程信息（名称/简介/专业分类/主线，归属校验）。
func (s *Server) teacherUpdateCourse(c *gin.Context) {
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
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course SET name = $1, intro = $2, line = $3, major_id = $4, updated_at = $5
		 WHERE course_code = $6 AND teacher_id = $7 AND status IN ('proposed','rejected')`,
		req.Name, req.Intro, line, req.MajorID, time.Now().UTC(), c.Param("code"), st.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10002, Msg: "课程不存在、非本人课程或状态不可编辑"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
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

// validCourseCode 校验平台课程码：字母/数字/连字符/下划线，2~32 位。
func validCourseCode(code string) bool {
	if len(code) < 2 || len(code) > 32 {
		return false
	}
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
