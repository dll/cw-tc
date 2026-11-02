package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func (s *Server) teacherOwnsCode(c *gin.Context, code string) (int64, bool) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	var courseID int64
	var owner sql.NullInt64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, teacher_id FROM course WHERE course_code=$1 AND deleted_at IS NULL`, code).
		Scan(&courseID, &owner); err != nil || !owner.Valid || owner.Int64 != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "只能访问本课数据"})
		return 0, false
	}
	return courseID, true
}

func (s *Server) teacherListInsights(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT i.id, i.course_code, CAST(i.period_start AS TEXT), CAST(i.period_end AS TEXT),
		        COALESCE(i.summary,''), i.verdict, COALESCE(i.verdict_note,''), i.generated_at
		 FROM course_insight i JOIN course c ON c.course_code=i.course_code
		 WHERE c.teacher_id=$1 AND ($2='' OR i.course_code=$2)
		 ORDER BY i.generated_at DESC, i.id DESC LIMIT 100`, st.ID, strings.TrimSpace(c.Query("course_code")))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "AI 周报读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var code, start, end, summary, verdict, note, generatedAt string
		if err := rows.Scan(&id, &code, &start, &end, &summary, &verdict, &note, &generatedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "AI 周报读取失败"})
			return
		}
		items = append(items, gin.H{"id": id, "course_code": code, "period_start": start, "period_end": end,
			"summary": summary, "verdict": verdict, "verdict_note": note, "generated_at": generatedAt})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) teacherInsightDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var code string
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT course_code FROM course_insight WHERE id=$1`, id).Scan(&code); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "报告不存在"})
		return
	}
	if _, ok := s.teacherOwnsCode(c, code); !ok {
		return
	}
	var start, end, summary, signals, clusters, suggestions, unknowns, evidence, modelInfo, generatedAt, verdict, note string
	var tokens int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT CAST(period_start AS TEXT), CAST(period_end AS TEXT), COALESCE(summary,''), signals, feedback_clusters,
		        suggestions, unknowns, evidence_map, COALESCE(model_info,'{}'), tokens, generated_at, verdict, COALESCE(verdict_note,'')
		 FROM course_insight WHERE id=$1`, id).
		Scan(&start, &end, &summary, &signals, &clusters, &suggestions, &unknowns, &evidence, &modelInfo, &tokens, &generatedAt, &verdict, &note); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "报告读取失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"id": id, "course_code": code, "period_start": start, "period_end": end, "summary": summary,
		"signals": jsonValue(signals), "feedback_clusters": jsonValue(clusters), "suggestions": jsonValue(suggestions),
		"unknowns": jsonValue(unknowns), "evidence_map": jsonValue(evidence), "model_info": jsonValue(modelInfo),
		"tokens": tokens, "generated_at": generatedAt, "verdict": verdict, "verdict_note": note,
	}})
}

func (s *Server) teacherInsightVerdict(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Verdict string `json:"verdict"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || (input.Verdict != "accepted" && input.Verdict != "rejected") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "verdict 只接受 accepted / rejected"})
		return
	}
	if input.Verdict == "rejected" && strings.TrimSpace(input.Note) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "驳回必须填写理由"})
		return
	}
	var code, verdict string
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT course_code, verdict FROM course_insight WHERE id=$1`, id).Scan(&code, &verdict); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "报告不存在"})
		return
	}
	if _, ok := s.teacherOwnsCode(c, code); !ok {
		return
	}
	if verdict != "pending" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "报告已给出结论"})
		return
	}
	st := c.MustGet(staffContextKey).(staff.Staff)
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course_insight SET verdict=$1, verdict_by=$2, verdict_at=$3, verdict_note=$4 WHERE id=$5 AND verdict='pending'`,
		input.Verdict, st.ID, time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(input.Note), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "报告结论保存失败"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role), Action: "insight.verdict",
		TargetType: "course_insight", TargetID: strconv.FormatInt(id, 10), After: input.Verdict, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"verdict": input.Verdict}})
}

func (s *Server) teacherListRevisionPlans(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT p.id, p.course_code, p.source, COALESCE(p.insight_id,0), p.problem, p.evidence, p.measure,
		        COALESCE(p.priority,''), p.target_metric, p.status, COALESCE(p.abandon_reason,''), p.created_at, p.updated_at
		 FROM revision_plan p JOIN course c ON c.course_code=p.course_code
		 WHERE c.teacher_id=$1 AND ($2='' OR p.course_code=$2) AND ($3='' OR p.status=$3)
		 ORDER BY p.updated_at DESC, p.id DESC LIMIT 100`, st.ID, strings.TrimSpace(c.Query("course_code")), strings.TrimSpace(c.Query("status")))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "改版计划读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, insightID int64
		var code, source, problem, evidence, measure, priority, target, status, abandon, created, updated string
		if err := rows.Scan(&id, &code, &source, &insightID, &problem, &evidence, &measure, &priority, &target, &status, &abandon, &created, &updated); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "改版计划读取失败"})
			return
		}
		items = append(items, gin.H{"id": id, "course_code": code, "source": source, "insight_id": insightID,
			"problem": problem, "evidence": evidence, "measure": measure, "priority": priority,
			"target_metric": target, "status": status, "abandon_reason": abandon, "created_at": created, "updated_at": updated})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) teacherCreateRevisionPlan(c *gin.Context) {
	var input struct {
		CourseCode   string `json:"course_code"`
		Source       string `json:"source"`
		InsightID    int64  `json:"insight_id"`
		Problem      string `json:"problem"`
		Evidence     string `json:"evidence"`
		Measure      string `json:"measure"`
		Priority     string `json:"priority"`
		TargetMetric string `json:"target_metric"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.CourseCode) == "" ||
		strings.TrimSpace(input.Problem) == "" || strings.TrimSpace(input.Evidence) == "" ||
		strings.TrimSpace(input.Measure) == "" || strings.TrimSpace(input.TargetMetric) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "问题 / 依据 / 措施 / 目标指标均必填"})
		return
	}
	if input.Source == "" {
		input.Source = "manual"
	}
	if input.Source != "ai" && input.Source != "manual" && input.Source != "student_feedback" && input.Source != "admin" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "source 非法"})
		return
	}
	if input.Priority != "" && input.Priority != "P0" && input.Priority != "P1" && input.Priority != "P2" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "priority 只接受 P0/P1/P2"})
		return
	}
	if _, ok := s.teacherOwnsCode(c, input.CourseCode); !ok {
		return
	}
	var insight any
	if input.InsightID > 0 {
		var code string
		if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT course_code FROM course_insight WHERE id=$1`, input.InsightID).Scan(&code); err != nil || code != input.CourseCode {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "关联报告不属于本课"})
			return
		}
		insight = input.InsightID
	}
	st := c.MustGet(staffContextKey).(staff.Staff)
	now := time.Now().UTC().Format(time.RFC3339)
	var id int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO revision_plan(course_code, source, insight_id, problem, evidence, measure, priority,
		   target_metric, owner_id, status, created_at, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'evaluating',$10,$10) RETURNING id`,
		input.CourseCode, input.Source, insight, strings.TrimSpace(input.Problem), strings.TrimSpace(input.Evidence),
		strings.TrimSpace(input.Measure), nullIfBlank(input.Priority), strings.TrimSpace(input.TargetMetric),
		st.ID, now).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "改版计划保存失败"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role), Action: "revision_plan.create",
		TargetType: "revision_plan", TargetID: strconv.FormatInt(id, 10), After: input.CourseCode, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

func (s *Server) teacherUpdateRevisionPlan(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Status        string `json:"status"`
		AbandonReason string `json:"abandon_reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	switch input.Status {
	case "accepted", "in_progress", "done", "abandoned":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 accepted / in_progress / done / abandoned"})
		return
	}
	var code string
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT course_code FROM revision_plan WHERE id=$1`, id).Scan(&code); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "改版计划不存在"})
		return
	}
	if _, ok := s.teacherOwnsCode(c, code); !ok {
		return
	}
	if input.Status == "abandoned" && strings.TrimSpace(input.AbandonReason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "放弃必须填写原因"})
		return
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE revision_plan SET status=$1, abandon_reason=$2, updated_at=$3 WHERE id=$4`,
		input.Status, nullIfBlank(input.AbandonReason), time.Now().UTC().Format(time.RFC3339), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "改版计划更新失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) teacherRevisionCompare(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var code, publishedAt string
	var version int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT course_code, CAST(published_at AS TEXT), content_version FROM course_revision WHERE id=$1`, id).
		Scan(&code, &publishedAt, &version); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "版本记录不存在"})
		return
	}
	if _, ok := s.teacherOwnsCode(c, code); !ok {
		return
	}
	at, err := time.Parse(time.RFC3339, publishedAt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "版本时间异常"})
		return
	}
	before := s.statsWindow(c, code, at.AddDate(0, 0, -30).Format("2006-01-02"), at.Format("2006-01-02"))
	after := s.statsWindow(c, code, at.Format("2006-01-02"), at.AddDate(0, 0, 30).Format("2006-01-02"))
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"course_code": code, "content_version": version, "published_at": publishedAt,
		"before": before, "after": after,
	}})
}

func (s *Server) statsWindow(c *gin.Context, code, from, to string) gin.H {
	var orders, paid, learning, completed, examined, certified int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COALESCE(SUM(orders),0), COALESCE(SUM(paid),0), COALESCE(SUM(learning),0),
		        COALESCE(SUM(completed),0), COALESCE(SUM(examined),0), COALESCE(SUM(certified),0)
		 FROM course_stat_daily WHERE course_code=$1 AND CAST(stat_date AS TEXT) >= $2 AND CAST(stat_date AS TEXT) < $3`,
		code, from, to).Scan(&orders, &paid, &learning, &completed, &examined, &certified)
	return gin.H{"from": from, "to": to, "orders": orders, "paid": paid, "learning": learning,
		"completed": completed, "examined": examined, "certified": certified}
}
