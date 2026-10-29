package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminCourses GET /admin/courses —— 全部课程（含草稿/已下架），支持状态、来源、关键词筛选。
// 同时返回页面顶部需要的简略统计（各状态数量、总课程数、总讲次数）。
func (s *Server) adminCourses(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	where := []string{"c.deleted_at IS NULL"}
	args := make([]any, 0)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(clause, "?", "$"+strconv.Itoa(len(args))))
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		add("c.status = ?", status)
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		add("c.source = ?", source)
	}
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		idx := "$" + strconv.Itoa(len(args))
		where = append(where, "(c.course_code LIKE "+idx+" OR c.name LIKE "+idx+" OR c.intro LIKE "+idx+")")
	}
	query := `SELECT c.course_code, c.source, c.name, c.intro, c.line, c.lecture_count, c.status,
	                 c.price_cents, c.teacher_id, c.created_at, c.updated_at, c.major_id, m.name,
	                 c.free_preview_lectures, c.sort_weight, c.reviewed_at, c.reject_reason,
	                 c.ai_enabled, c.kb_status
	          FROM course c LEFT JOIN major m ON m.id = c.major_id
	          WHERE ` + strings.Join(where, " AND ") + ` ORDER BY c.id DESC`
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, source, name, line, status, createdAt, updatedAt, kbStatus string
		var intro, majorName, reviewedAt, rejectReason sql.NullString
		var priceCents int64
		var teacherID, majorID *int64
		var lectureCount, freePreviewLectures, sortWeight int
		var aiEnabled bool
		if err := rows.Scan(&code, &source, &name, &intro, &line, &lectureCount, &status,
			&priceCents, &teacherID, &createdAt, &updatedAt, &majorID, &majorName,
			&freePreviewLectures, &sortWeight, &reviewedAt, &rejectReason, &aiEnabled, &kbStatus); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"course_code": code, "source": source, "name": name, "intro": nullableString(intro),
			"line": line, "lecture_count": lectureCount, "status": status,
			"price_cents": priceCents, "teacher_id": teacherID, "created_at": createdAt,
			"updated_at": updatedAt, "major_id": majorID, "major_name": nullableString(majorName),
			"free_preview_lectures": freePreviewLectures, "sort_weight": sortWeight,
			"reviewed_at": nullableString(reviewedAt), "reject_reason": nullableString(rejectReason),
			"ai_enabled": aiEnabled, "kb_status": kbStatus,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}

	// 顶部统计：各状态数量 + 总课程数 + 总讲次数（不受筛选条件影响）。
	stats := map[string]int{}
	total, totalLectures := 0, 0
	statRows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT status, COUNT(*), COALESCE(SUM(lecture_count),0) FROM course WHERE deleted_at IS NULL GROUP BY status`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer statRows.Close()
	for statRows.Next() {
		var status string
		var count, lectures int
		if err := statRows.Scan(&status, &count, &lectures); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		stats[status] = count
		total += count
		totalLectures += lectures
	}
	if err := statRows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items":          items,
		"stats":          stats,
		"total":          total,
		"total_lectures": totalLectures,
	}})
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
	admin := c.MustGet(staffContextKey).(staff.Staff)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.major_change", TargetType: "course", TargetID: c.Param("code"),
		After: strconv.FormatInt(in.MajorID, 10), IP: c.ClientIP(),
	})
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
	admin := c.MustGet(staffContextKey).(staff.Staff)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.free_preview_change", TargetType: "course", TargetID: c.Param("code"),
		After: strconv.Itoa(in.Count), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": c.Param("code"), "free_preview_lectures": in.Count}})
}
