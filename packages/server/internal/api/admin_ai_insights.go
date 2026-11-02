package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/aigateway"
	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/internal/stats"
	"github.com/gin-gonic/gin"
)

type insightSuggestion struct {
	Problem      string   `json:"problem"`
	Measure      string   `json:"measure"`
	Priority     string   `json:"priority"`
	TargetMetric string   `json:"target_metric"`
	Evidence     []string `json:"evidence"`
}

type insightPayload struct {
	Summary          string              `json:"summary"`
	Signals          []map[string]any    `json:"signals"`
	FeedbackClusters []map[string]any    `json:"feedback_clusters"`
	Suggestions      []insightSuggestion `json:"suggestions"`
	Unknowns         []string            `json:"unknowns"`
}

// adminGenerateAIInsight POST /admin/courses/:code/ai-insights/generate —— AI 运营报告（每条建议可溯源）。
func (s *Server) adminGenerateAIInsight(c *gin.Context) {
	code := c.Param("code")
	gateway := s.aiGateway()
	if gateway == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10601, Msg: "AI 运营报告未配置模型备案号"})
		return
	}
	var courseID int64
	var courseName string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, name FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).
		Scan(&courseID, &courseName); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	stat, err := stats.Latest(c.Request.Context(), s.dbConn, code)
	if err != nil || stat == nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "尚无 T+1 统计，先跑批再生成报告"})
		return
	}
	evidence := map[string]any{
		"stats.views_uv": stat.ViewsUV, "stats.favorites": stat.Favorites,
		"stats.learning": stat.Learning, "stats.completed": stat.Completed,
		"stats.examined": stat.Examined, "stats.certified": stat.Certified,
		"stats.data_as_of": stat.StatDate,
	}
	commentRows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, rating, COALESCE(body,'') FROM course_comment
		 WHERE course_code=$1 AND status='visible' ORDER BY created_at DESC LIMIT 30`, code)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价数据读取失败"})
		return
	}
	comments := make([]gin.H, 0)
	for commentRows.Next() {
		var id int64
		var rating int
		var body string
		if commentRows.Scan(&id, &rating, &body) == nil {
			key := fmt.Sprintf("comment.%d", id)
			evidence[key] = gin.H{"rating": rating, "body": body}
			comments = append(comments, gin.H{"key": key, "rating": rating, "body": body})
		}
	}
	commentRows.Close()
	topicRows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT t.id, t.body FROM lecture_topic t JOIN lecture l ON l.id=t.lecture_id
		 WHERE l.course_id=$1 ORDER BY t.created_at DESC LIMIT 30`, courseID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论数据读取失败"})
		return
	}
	topics := make([]gin.H, 0)
	for topicRows.Next() {
		var id int64
		var body string
		if topicRows.Scan(&id, &body) == nil {
			key := fmt.Sprintf("topic.%d", id)
			evidence[key] = body
			topics = append(topics, gin.H{"key": key, "body": body})
		}
	}
	topicRows.Close()
	var refunds int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM refund r JOIN orders o ON o.id=r.order_id
		 WHERE o.course_id=$1 AND r.finished_at >= $2`,
		courseID, time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)).Scan(&refunds)
	evidence["refunds.recent_count"] = refunds
	var anomalies int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM exam_anomaly a JOIN exam_session e ON e.id=a.exam_id
		 WHERE e.course_id=$1 AND a.verdict='pending'`, courseID).Scan(&anomalies)
	evidence["anomalies.pending_count"] = anomalies
	evidence["stats.orders"] = stat.Orders
	evidence["stats.paid"] = stat.Paid
	evidence["stats.unlock"] = stat.Unlock
	evidence["comments.count"] = len(comments)
	if stat.RatingCount > 0 && stat.AvgRating != nil {
		evidence["comments.avg_rating"] = *stat.AvgRating
	}
	evidence["topics.count"] = len(topics)
	var unanswered int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM lecture_topic t JOIN lecture l ON l.id=t.lecture_id
		 WHERE l.course_id=$1 AND NOT EXISTS(SELECT 1 FROM lecture_reply r WHERE r.topic_id=t.id)
		   AND NOT EXISTS(SELECT 1 FROM lecture_reply_staff rs WHERE rs.topic_id=t.id)`, courseID).Scan(&unanswered)
	evidence["topics.unanswered"] = unanswered
	var aiUsage int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COALESCE(SUM(used),0) FROM ai_daily_quota WHERE course_id=$1`, courseID).Scan(&aiUsage)
	evidence["ai.usage_total"] = aiUsage
	var publishedDrafts int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM ai_content_draft WHERE course_code=$1 AND status='published'`, code).Scan(&publishedDrafts)
	evidence["ai_drafts.published_count"] = publishedDrafts
	var latestDraftID, latestDraftVersion int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, content_version FROM ai_content_draft
		 WHERE course_code=$1 AND status='published' AND content_version IS NOT NULL
		 ORDER BY id DESC LIMIT 1`, code).Scan(&latestDraftID, &latestDraftVersion); err == nil {
		evidence["ai_drafts.latest_id"] = latestDraftID
		var publishedAt string
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT CAST(published_at AS TEXT) FROM course_revision
			 WHERE course_code=$1 AND content_version=$2 ORDER BY revision_no DESC LIMIT 1`,
			code, latestDraftVersion).Scan(&publishedAt); err == nil {
			if at, err := time.Parse(time.RFC3339, publishedAt); err == nil {
				before := s.statsWindow(c, code, at.AddDate(0, 0, -30).Format("2006-01-02"), at.Format("2006-01-02"))
				after := s.statsWindow(c, code, at.Format("2006-01-02"), at.AddDate(0, 0, 30).Format("2006-01-02"))
				for key, value := range before {
					evidence[fmt.Sprintf("ai_draft.%d.before.%s", latestDraftID, key)] = value
				}
				for key, value := range after {
					evidence[fmt.Sprintf("ai_draft.%d.after.%s", latestDraftID, key)] = value
				}
				evidence[fmt.Sprintf("ai_draft.%d.published_at", latestDraftID)] = publishedAt
			}
		}
	}
	evidenceJSON, _ := json.Marshal(evidence)
	prompt := fmt.Sprintf("课程：%s\n可用证据（建议只能引用这些 key）：%s\n评价：%s\n讨论：%s\n请只输出 JSON：{\"summary\":\"...\",\"signals\":[{\"key\":\"...\",\"text\":\"...\"}],\"feedback_clusters\":[{\"label\":\"...\",\"count\":1,\"comment_ids\":[]}],\"suggestions\":[{\"problem\":\"...\",\"measure\":\"...\",\"priority\":\"P1\",\"target_metric\":\"...\",\"evidence\":[\"stats.learning\"]}],\"unknowns\":[\"...\"]}。每条建议必须至少引用一个证据 key，不得编造。",
		courseName, string(evidenceJSON), maybeJSON(comments), maybeJSON(topics))
	resp, err := gateway.Complete(c.Request.Context(), "assistant.strong", aigateway.Request{
		System: "你是课程运营分析助手。只依据给定指标、评价、讨论生成可执行建议；每条建议必须给出证据 key；不确定的写入 unknowns，不得编造数据。",
		Prompt: prompt, Temperature: 0.2, MaxTokens: 1600,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 运营报告生成失败"})
		return
	}
	payload, err := parseInsightPayload(resp.Content, evidence)
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: err.Error()})
		return
	}
	signalsJSON, _ := json.Marshal(payload.Signals)
	clustersJSON, _ := json.Marshal(payload.FeedbackClusters)
	suggestionsJSON, _ := json.Marshal(payload.Suggestions)
	unknownsJSON, _ := json.Marshal(payload.Unknowns)
	evidenceMap := map[int][]string{}
	for idx, suggestion := range payload.Suggestions {
		evidenceMap[idx] = suggestion.Evidence
	}
	evidenceMapJSON, _ := json.Marshal(evidenceMap)
	modelInfo, _ := json.Marshal(gin.H{
		"alias": resp.Alias, "model": resp.Model, "model_version": resp.ModelVersion, "filing_no": resp.FilingNo,
	})
	now := time.Now().UTC()
	periodStart := now.AddDate(0, 0, -7).Format("2006-01-02")
	periodEnd := now.Format("2006-01-02")
	var id int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO course_insight(course_code, period_start, period_end, summary, signals, feedback_clusters,
		   suggestions, unknowns, evidence_map, model_info, tokens, generated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		code, periodStart, periodEnd, payload.Summary, string(signalsJSON), string(clustersJSON),
		string(suggestionsJSON), string(unknownsJSON), string(evidenceMapJSON), string(modelInfo),
		resp.TokensIn+resp.TokensOut, now.Format(time.RFC3339)).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "AI 运营报告保存失败"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	after, _ := json.Marshal(gin.H{"insight_id": id, "suggestions": len(payload.Suggestions), "filing_no": resp.FilingNo})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "ai.insight_generate",
		TargetType: "course", TargetID: code, After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"id": id, "summary": payload.Summary, "signals": payload.Signals,
		"feedback_clusters": payload.FeedbackClusters, "suggestions": payload.Suggestions,
		"unknowns": payload.Unknowns, "evidence_map": evidenceMap,
		"evidence": evidence,
		"model":    resp.Model, "model_version": resp.ModelVersion, "filing_no": resp.FilingNo,
		"generated_at": now.Format(time.RFC3339),
	}})
}

func parseInsightPayload(raw string, evidence map[string]any) (insightPayload, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var payload insightPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &payload); err != nil {
		return payload, fmt.Errorf("AI 报告结构非法")
	}
	if strings.TrimSpace(payload.Summary) == "" || len(payload.Suggestions) == 0 {
		return payload, fmt.Errorf("AI 报告缺少摘要或建议")
	}
	for _, suggestion := range payload.Suggestions {
		if suggestion.Problem == "" || suggestion.Measure == "" || suggestion.TargetMetric == "" || len(suggestion.Evidence) == 0 {
			return payload, fmt.Errorf("AI 建议必须填写问题 / 措施 / 目标指标 / 证据")
		}
		for _, key := range suggestion.Evidence {
			if _, ok := evidence[key]; !ok {
				return payload, fmt.Errorf("AI 建议引用了不存在的证据：%s", key)
			}
		}
	}
	return payload, nil
}

func maybeJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func (s *Server) adminListAIInsights(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, period_start, period_end, COALESCE(summary,''), suggestions, evidence_map, model_info, tokens, generated_at
		 FROM course_insight WHERE course_code=$1 ORDER BY generated_at DESC, id DESC LIMIT 30`, c.Param("code"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "AI 运营报告读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var start, end, summary, suggestions, evidenceMap, modelInfo, generatedAt string
		var tokens int
		if err := rows.Scan(&id, &start, &end, &summary, &suggestions, &evidenceMap, &modelInfo, &tokens, &generatedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "AI 运营报告读取失败"})
			return
		}
		items = append(items, gin.H{"id": id, "period_start": start, "period_end": end, "summary": summary,
			"suggestions": jsonValue(suggestions), "evidence_map": jsonValue(evidenceMap),
			"model_info": jsonValue(modelInfo), "tokens": tokens, "generated_at": generatedAt})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminAIInsightDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "报告编号非法"})
		return
	}
	var start, end, summary, signals, clusters, suggestions, unknowns, evidenceMap, modelInfo, generatedAt string
	var tokens int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT CAST(period_start AS TEXT), CAST(period_end AS TEXT), COALESCE(summary,''), signals, feedback_clusters, suggestions, unknowns, evidence_map, model_info, tokens, generated_at
		 FROM course_insight WHERE id=$1 AND course_code=$2`, id, c.Param("code")).
		Scan(&start, &end, &summary, &signals, &clusters, &suggestions, &unknowns, &evidenceMap, &modelInfo, &tokens, &generatedAt); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "报告不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"id": id, "period_start": start, "period_end": end, "summary": summary,
		"signals": jsonValue(signals), "feedback_clusters": jsonValue(clusters),
		"suggestions": jsonValue(suggestions), "unknowns": jsonValue(unknowns),
		"evidence_map": jsonValue(evidenceMap), "model_info": jsonValue(modelInfo),
		"tokens": tokens, "generated_at": generatedAt,
	}})
}

func jsonValue(raw string) any {
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		return value
	}
	return raw
}
