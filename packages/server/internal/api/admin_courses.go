package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// adminCourses GET /admin/courses —— 全部课程（含未上架/待审），按状态过滤可选。
func (s *Server) adminCourses(c *gin.Context) {
	query := `SELECT c.course_code, c.source, c.name, c.line, c.status, c.price_cents, c.teacher_id, c.created_at, c.major_id, m.name, c.free_preview_lectures FROM course c LEFT JOIN major m ON m.id = c.major_id`
	args := make([]any, 0)
	if st := c.Query("status"); st != "" {
		query += ` WHERE c.status = $1`
		args = append(args, st)
	}
	query += ` ORDER BY c.id DESC`
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
		var teacherID, majorID *int64
		var majorName *string
		var freePreviewLectures int
		if err := rows.Scan(&code, &source, &name, &line, &status, &priceCents, &teacherID, &createdAt, &majorID, &majorName, &freePreviewLectures); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"course_code": code, "source": source, "name": name, "line": line,
			"status": status, "price_cents": priceCents, "teacher_id": teacherID, "created_at": createdAt,
			"major_id": majorID, "major_name": majorName, "free_preview_lectures": freePreviewLectures,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminSetCourseMajor POST /admin/courses/:code/major —— 给课程设置专业分类（平台自有数据，铁律 1）。
func (s *Server) adminSetCourseMajor(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	var in struct {
		MajorID int64 `json:"major_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.MajorID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var exists int
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM major WHERE id = $1`, in.MajorID).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "专业分类不存在"})
		return
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(), `UPDATE course SET major_id = $1 WHERE course_code = $2`, in.MajorID, c.Param("code"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": c.Param("code"), "major_id": in.MajorID}})
}

// adminSetCourseFreePreview POST /admin/courses/:code/free-preview —— 设置免费预览讲次数（平台自有配置）。
func (s *Server) adminSetCourseFreePreview(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	var in struct {
		Count int `json:"count"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Count < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(), `UPDATE course SET free_preview_lectures = $1 WHERE course_code = $2`, in.Count, c.Param("code"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": c.Param("code"), "free_preview_lectures": in.Count}})
}
