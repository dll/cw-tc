package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	                 c.ai_enabled, c.kb_status, c.content_version, c.published_version
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
		var teacherID, majorID, publishedVersion *int64
		var lectureCount, freePreviewLectures, sortWeight, contentVersion int
		var aiEnabled bool
		if err := rows.Scan(&code, &source, &name, &intro, &line, &lectureCount, &status,
			&priceCents, &teacherID, &createdAt, &updatedAt, &majorID, &majorName,
			&freePreviewLectures, &sortWeight, &reviewedAt, &rejectReason, &aiEnabled, &kbStatus,
			&contentVersion, &publishedVersion); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		hasUnpublishedChanges := publishedVersion != nil && int(*publishedVersion) != contentVersion
		items = append(items, gin.H{
			"course_code": code, "source": source, "name": name, "intro": nullableString(intro),
			"line": line, "lecture_count": lectureCount, "status": status,
			"price_cents": priceCents, "teacher_id": teacherID, "created_at": createdAt,
			"updated_at": updatedAt, "major_id": majorID, "major_name": nullableString(majorName),
			"free_preview_lectures": freePreviewLectures, "sort_weight": sortWeight,
			"reviewed_at": nullableString(reviewedAt), "reject_reason": nullableString(rejectReason),
			"ai_enabled": aiEnabled, "kb_status": kbStatus,
			"content_version": contentVersion, "published_version": publishedVersion,
			"has_unpublished_changes": hasUnpublishedChanges,
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

// adminCourseRevisions GET /admin/courses/:code/revisions —— 课程改版（发布）历史。
func (s *Server) adminCourseRevisions(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT revision_no, content_version, changed_digest, published_at FROM course_revision WHERE course_code = $1 ORDER BY revision_no DESC`,
		c.Param("code"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var revisionNo, contentVersion int
		var digest, publishedAt string
		if err := rows.Scan(&revisionNo, &contentVersion, &digest, &publishedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{
			"revision_no": revisionNo, "content_version": contentVersion,
			"changed_digest": digest, "published_at": publishedAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminCourseQuality GET /admin/courses/:code/quality —— 质量分、AI 等级与最近一次审核。
func (s *Server) adminCourseQuality(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	var quality sql.NullFloat64
	var aiLevel, kbStatus, status sql.NullString
	var aiEnabled bool
	var contentVersion int
	var publishedVersion *int64
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT CAST(quality_score AS REAL), ai_level, ai_enabled, kb_status, status, content_version, published_version
		 FROM course WHERE course_code = $1 AND deleted_at IS NULL`, c.Param("code")).
		Scan(&quality, &aiLevel, &aiEnabled, &kbStatus, &status, &contentVersion, &publishedVersion)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	data := gin.H{
		"course_code": c.Param("code"), "status": status.String,
		"quality_score": nil, "ai_level": nil,
		"ai_enabled": aiEnabled, "kb_status": kbStatus.String,
		"content_version": contentVersion, "published_version": publishedVersion,
		"latest_review": nil,
	}
	if quality.Valid {
		data["quality_score"] = quality.Float64
	}
	if aiLevel.Valid {
		data["ai_level"] = aiLevel.String
	}

	var round, reviewContentVersion int
	var reviewerID int64
	var reviewerRole, reviewerName, verdict, issueList, reviewedAt sql.NullString
	var scoreMaterial, scoreObjective, scoreCase, scoreLessonPlan, scoreSlides, scoreLab, totalScore sql.NullFloat64
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT r.round, r.content_version, r.reviewer_id, r.reviewer_role, st.name,
		        CAST(r.score_material AS REAL), CAST(r.score_objective AS REAL), CAST(r.score_case AS REAL),
		        CAST(r.score_lesson_plan AS REAL), CAST(r.score_slides AS REAL), CAST(r.score_lab AS REAL),
		        CAST(r.total_score AS REAL), r.verdict, CAST(r.issue_list AS TEXT), r.reviewed_at
		 FROM course_review r LEFT JOIN staff st ON st.id = r.reviewer_id
		 WHERE r.course_code = $1 ORDER BY r.round DESC, r.reviewed_at DESC LIMIT 1`, c.Param("code")).
		Scan(&round, &reviewContentVersion, &reviewerID, &reviewerRole, &reviewerName,
			&scoreMaterial, &scoreObjective, &scoreCase, &scoreLessonPlan, &scoreSlides, &scoreLab,
			&totalScore, &verdict, &issueList, &reviewedAt)
	if err == nil {
		data["latest_review"] = gin.H{
			"round": round, "content_version": reviewContentVersion,
			"reviewer_id": reviewerID, "reviewer_role": reviewerRole.String,
			"reviewer_name":  nullableString(reviewerName),
			"score_material": scoreMaterial.Float64, "score_objective": scoreObjective.Float64,
			"score_case": scoreCase.Float64, "score_lesson_plan": scoreLessonPlan.Float64,
			"score_slides": scoreSlides.Float64, "score_lab": scoreLab.Float64,
			"total_score": totalScore.Float64, "verdict": verdict.String,
			"issue_list": nullableString(issueList), "reviewed_at": reviewedAt.String,
		}
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: data})
}

// adminCourseQualityOverride POST /admin/courses/:code/quality —— 核定覆写质量分 / AI 等级（写审计）。
func (s *Server) adminCourseQualityOverride(c *gin.Context) {
	var req struct {
		QualityScore *float64 `json:"quality_score"`
		AILevel      string   `json:"ai_level"`
		Reason       string   `json:"reason"`
		Confirm      bool     `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm || strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "事由必填且需二次确认"})
		return
	}
	if req.QualityScore == nil && req.AILevel == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "质量分与 AI 等级至少填写一项"})
		return
	}
	if req.QualityScore != nil && (*req.QualityScore < 0 || *req.QualityScore > 100) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "质量分应在 0~100"})
		return
	}
	if req.AILevel != "" && req.AILevel != "LA" && req.AILevel != "LB" && req.AILevel != "LC" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "AI 等级只能为 LA / LB / LC"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course SET quality_score = COALESCE($1, quality_score), ai_level = COALESCE(NULLIF($2,''), ai_level), updated_at = $3
		 WHERE course_code = $4 AND deleted_at IS NULL`,
		req.QualityScore, req.AILevel, time.Now().UTC().Format(time.RFC3339), c.Param("code"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	after, _ := json.Marshal(map[string]any{"quality_score": req.QualityScore, "ai_level": req.AILevel, "reason": req.Reason})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.quality_override", TargetType: "course", TargetID: c.Param("code"),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
}
