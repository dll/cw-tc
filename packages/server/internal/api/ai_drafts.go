package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/aicontent"
	"github.com/cw-tc/server/internal/aigateway"
	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

type aiDraft struct {
	ID              int64
	CourseCode      string
	CourseName      string
	LectureNo       sql.NullInt64
	DraftScope      string
	LecturesJSON    string
	LectureCount    int
	CitationsJSON   string
	AutoCheckJSON   sql.NullString
	AutoCheckPassed bool
	ModelID         string
	ModelVersion    string
	FilingNo        string
	AIGCLabeled     bool
	AIGCMetaWritten bool
	Kind            string
	TriggerKind     string
	TriggerTopics   string
	ContentMD       string
	Tokens          int
	Status          string
	RejectCount     int
	ContentVersion  sql.NullInt64
	CreatedAt       string
	UpdatedAt       string
	TeacherID       sql.NullInt64
}

const aiDraftQuery = `SELECT id, course_code, lecture_no, draft_scope, lectures, lecture_count,
	source_citations, auto_check, auto_check_passed, model_id, model_version, filing_no,
	aigc_labeled, aigc_meta_written, kind, trigger_kind, trigger_topic_ids, content_md, tokens,
	status, reject_count, content_version, created_at, updated_at
	FROM ai_content_draft`

func scanAIDraft(scanner interface{ Scan(...any) error }) (aiDraft, error) {
	var d aiDraft
	var lectures, citations []byte
	var autoCheck []byte
	if err := scanner.Scan(&d.ID, &d.CourseCode, &d.LectureNo, &d.DraftScope, &lectures, &d.LectureCount,
		&citations, &autoCheck, &d.AutoCheckPassed, &d.ModelID, &d.ModelVersion, &d.FilingNo,
		&d.AIGCLabeled, &d.AIGCMetaWritten, &d.Kind, &d.TriggerKind, &d.TriggerTopics, &d.ContentMD,
		&d.Tokens, &d.Status, &d.RejectCount, &d.ContentVersion, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return d, err
	}
	d.LecturesJSON = string(lectures)
	d.CitationsJSON = string(citations)
	d.AutoCheckJSON = sql.NullString{String: string(autoCheck), Valid: len(autoCheck) > 0}
	return d, nil
}

func (s *Server) aiDraftOwned(c *gin.Context, id int64) (aiDraft, bool) {
	draft, err := scanAIDraft(s.dbConn.QueryRowContext(c.Request.Context(), aiDraftQuery+` WHERE id = $1`, id))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "草稿不存在"})
		return draft, false
	}
	st := c.MustGet(staffContextKey).(staff.Staff)
	var owner sql.NullInt64
	_ = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT teacher_id FROM course WHERE course_code = $1`, draft.CourseCode).Scan(&owner)
	if !owner.Valid || owner.Int64 != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "只能操作本课草稿"})
		return draft, false
	}
	return draft, true
}

func (s *Server) teacherCreateAIDraft(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	var input struct {
		CourseCode     string  `json:"course_code"`
		LectureNo      *int    `json:"lecture_no"`
		DraftScope     string  `json:"draft_scope"`
		Kind           string  `json:"kind"`
		TriggerKind    string  `json:"trigger_kind"`
		SourceTopicIDs []int64 `json:"source_topic_ids"`
		LectureCount   int     `json:"lecture_count"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.CourseCode) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if input.DraftScope == "" {
		input.DraftScope = "single"
	}
	if input.TriggerKind == "" {
		input.TriggerKind = "teacher_mark"
	}
	if input.Kind == "" {
		input.Kind = "new_lecture"
	}
	if input.DraftScope != "single" && input.DraftScope != "chapter" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "draft_scope 只接受 single / chapter"})
		return
	}
	if input.LectureCount <= 0 {
		input.LectureCount = 1
	}
	if input.DraftScope == "single" {
		input.LectureCount = 1
	}
	if input.LectureCount > aicontent.MaxLectures {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "单次生成讲次数超过上限"})
		return
	}
	if len(input.SourceTopicIDs) == 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "生成必须有触发来源（建议原帖）"})
		return
	}
	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id FROM course WHERE course_code = $1 AND source = 'platform' AND teacher_id = $2 AND deleted_at IS NULL`,
		input.CourseCode, st.ID).Scan(&courseID); err != nil {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "只能为本人的平台课程生成"})
		return
	}
	topics, err := s.loadDraftTopics(c, courseID, input.SourceTopicIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "触发建议不存在或不属于本课"})
		return
	}
	gateway := s.aiGateway()
	if gateway == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10601, Msg: "AI 生成未配置模型备案号"})
		return
	}
	lectureNo := 1
	if input.LectureNo != nil && *input.LectureNo > 0 {
		lectureNo = *input.LectureNo
	}
	contextText, citations := s.courseContext(c.Request.Context(), courseID, lectureNo, topics)
	if len(citations) == 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10601, Msg: "本课暂无可引用资料，拒绝生成"})
		return
	}
	prompt := fmt.Sprintf("课程码：%s\n触发建议：%s\n课程资料：\n%s\n请生成 %d 讲 Markdown 草稿。只输出 JSON：{\"lectures\":[{\"title\":\"...\",\"content_md\":\"...\"}]}，不得生成测评题目或答案。",
		input.CourseCode, topics, contextText, input.LectureCount)
	resp, err := gateway.Complete(c.Request.Context(), "assistant.strong", aigateway.Request{
		System: "你是课程内容生成助手。只能依据提供的课程资料生成平台增补讲次；必须可追溯到资料，不得编造文献/标准/实验数据，不得生成题目答案。",
		Prompt: prompt, Temperature: 0.3, MaxTokens: 2000,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 生成失败"})
		return
	}
	lectures, err := aicontent.ParseLectures(resp.Content, input.LectureCount)
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 草稿结构不合法：" + err.Error()})
		return
	}
	lectures = aicontent.WithAILabel(lectures)
	passed, report := aicontent.AutoCheck(lectures, len(citations))
	reportJSON, _ := json.Marshal(report)
	citationsJSON, _ := json.Marshal(citations)
	lecturesJSON, _ := json.Marshal(lectures)
	topicJSON, _ := json.Marshal(input.SourceTopicIDs)
	status := "pending_teacher"
	if !passed {
		status = "drafting"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var id int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO ai_content_draft(course_code, lecture_no, draft_scope, lectures, lecture_count,
		   source_citations, auto_check, auto_check_passed, model_id, model_version, filing_no,
		   aigc_labeled, aigc_meta_written, kind, trigger_kind, trigger_topic_ids, content_md,
		   tokens, ai_labeled, status, created_at, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,'assistant.strong',$9,$10,true,false,$11,$12,$13,$14,$15,true,$16,$17,$17)
		 RETURNING id`,
		input.CourseCode, input.LectureNo, input.DraftScope, string(lecturesJSON), input.LectureCount,
		string(citationsJSON), string(reportJSON), passed, resp.ModelVersion, resp.FilingNo,
		input.Kind, input.TriggerKind, string(topicJSON), aicontent.ContentMD(lectures),
		resp.TokensIn+resp.TokensOut, status, now).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "草稿保存失败"})
		return
	}
	after, _ := json.Marshal(gin.H{"draft_id": id, "status": status, "model": resp.Model, "filing_no": resp.FilingNo, "auto_check_passed": passed})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role), Action: "ai.draft_generate",
		TargetType: "ai_content_draft", TargetID: strconv.FormatInt(id, 10), After: string(after), IP: c.ClientIP(),
	})
	draft, _ := scanAIDraft(s.dbConn.QueryRowContext(c.Request.Context(), aiDraftQuery+` WHERE id = $1`, id))
	c.JSON(http.StatusOK, envelope{Code: 0, Data: draftPayload(draft, report, citations)})
}

func (s *Server) loadDraftTopics(c *gin.Context, courseID int64, ids []int64) (string, error) {
	if len(ids) == 0 {
		return "", errors.New("empty")
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, courseID)
	for i, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, "$"+strconv.Itoa(i+2))
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT t.body FROM lecture_topic t JOIN lecture l ON l.id = t.lecture_id
		 WHERE l.course_id = $1 AND t.id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	parts := make([]string, 0)
	for rows.Next() {
		var body string
		if rows.Scan(&body) == nil {
			parts = append(parts, body)
		}
	}
	if len(parts) != len(ids) {
		return "", errors.New("topic mismatch")
	}
	return strings.Join(parts, "\n"), nil
}

func draftPayload(draft aiDraft, autoCheck any, citations any) gin.H {
	payload := gin.H{
		"id": draft.ID, "course_code": draft.CourseCode, "draft_scope": draft.DraftScope,
		"lecture_count": draft.LectureCount, "status": draft.Status,
		"auto_check_passed": draft.AutoCheckPassed, "auto_check": autoCheck,
		"source_citations": citations, "model_id": draft.ModelID,
		"model_version": draft.ModelVersion, "filing_no": draft.FilingNo,
		"aigc_labeled": draft.AIGCLabeled, "aigc_meta_written": draft.AIGCMetaWritten,
		"content_md": draft.ContentMD, "created_at": draft.CreatedAt, "updated_at": draft.UpdatedAt,
	}
	if draft.LectureNo.Valid {
		payload["lecture_no"] = draft.LectureNo.Int64
	}
	if draft.AutoCheckJSON.Valid {
		var raw any
		if json.Unmarshal([]byte(draft.AutoCheckJSON.String), &raw) == nil {
			payload["auto_check"] = raw
		}
	}
	var raw any
	if json.Unmarshal([]byte(draft.LecturesJSON), &raw) == nil {
		payload["lectures"] = raw
	}
	if json.Unmarshal([]byte(draft.CitationsJSON), &raw) == nil {
		payload["source_citations"] = raw
	}
	return payload
}

func (s *Server) teacherListAIDrafts(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	status := c.DefaultQuery("status", "")
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		aiDraftQuery+` WHERE course_code IN (SELECT course_code FROM course WHERE teacher_id = $1)
		 AND ($2 = '' OR status = $2) ORDER BY updated_at DESC LIMIT 100`, st.ID, status)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "草稿列表读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		draft, err := scanAIDraft(rows)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "草稿列表读取失败"})
			return
		}
		items = append(items, draftPayload(draft, nil, nil))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) teacherAIDraftDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, ok := s.aiDraftOwned(c, id)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: draftPayload(draft, nil, nil)})
}

func (s *Server) teacherUpdateAIDraft(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, ok := s.aiDraftOwned(c, id)
	if !ok {
		return
	}
	if draft.Status == "published" || draft.Status == "abandoned" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "当前状态不可编辑"})
		return
	}
	var input struct {
		Lectures []aicontent.Lecture `json:"lectures"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || len(input.Lectures) != draft.LectureCount {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "讲次数必须与草稿声明一致"})
		return
	}
	lectures := aicontent.WithAILabel(input.Lectures)
	var citations []any
	_ = json.Unmarshal([]byte(draft.CitationsJSON), &citations)
	passed, report := aicontent.AutoCheck(lectures, len(citations))
	reportJSON, _ := json.Marshal(report)
	lecturesJSON, _ := json.Marshal(lectures)
	status := "pending_teacher"
	if !passed {
		status = "drafting"
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE ai_content_draft SET lectures=$1, content_md=$2, auto_check=$3, auto_check_passed=$4,
		        status=$5, updated_at=$6 WHERE id=$7`,
		string(lecturesJSON), aicontent.ContentMD(lectures), string(reportJSON), passed,
		status, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "草稿保存失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": status, "auto_check_passed": passed}})
}

func (s *Server) teacherReviewAIDraft(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, ok := s.aiDraftOwned(c, id)
	if !ok {
		return
	}
	var input struct {
		Verdict         string   `json:"verdict"`
		IssueList       []string `json:"issue_list"`
		EditedByTeacher bool     `json:"edited_by_teacher"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || (input.Verdict != "approved" && input.Verdict != "rejected") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "verdict 只接受 approved / rejected"})
		return
	}
	if input.Verdict == "rejected" && len(input.IssueList) == 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "驳回必须填写问题清单"})
		return
	}
	if draft.Status != "pending_teacher" || !draft.AutoCheckPassed {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "草稿未通过体检或不在待一审状态"})
		return
	}
	lectures, err := aicontent.ParseLectures(draft.LecturesJSON, draft.LectureCount)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "草稿内容损坏"})
		return
	}
	reviewer := c.MustGet(staffContextKey).(staff.Staff)
	issues, _ := json.Marshal(input.IssueList)
	nextStatus := "pending_admin"
	if input.Verdict == "rejected" {
		nextStatus = "teacher_rejected"
	}
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "一审提交失败"})
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(c.Request.Context(),
		`INSERT INTO ai_content_review(draft_id, stage, reviewer_id, verdict, issue_list,
		   edited_by_teacher, reviewed_content_hash, reviewed_at) VALUES($1,'teacher',$2,$3,$4,$5,$6,$7)`,
		id, reviewer.ID, input.Verdict, string(issues), input.EditedByTeacher,
		aicontent.Hash(lectures), time.Now().UTC().Format(time.RFC3339)); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "该版本已有一审结论"})
		return
	}
	rejectCount := draft.RejectCount
	if input.Verdict == "rejected" {
		rejectCount++
	}
	if _, err := tx.ExecContext(c.Request.Context(),
		`UPDATE ai_content_draft SET status=$1, reject_count=$2, updated_at=$3 WHERE id=$4`,
		nextStatus, rejectCount, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "一审提交失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "一审提交失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": nextStatus}})
}

func (s *Server) adminListAIDrafts(c *gin.Context) {
	status := c.DefaultQuery("status", "pending_admin")
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		aiDraftQuery+` WHERE ($1 = '' OR status = $1) ORDER BY updated_at DESC LIMIT 100`, status)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "二审队列读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		draft, err := scanAIDraft(rows)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "二审队列读取失败"})
			return
		}
		items = append(items, draftPayload(draft, nil, nil))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminAIDraftDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, err := scanAIDraft(s.dbConn.QueryRowContext(c.Request.Context(), aiDraftQuery+` WHERE id = $1`, id))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "草稿不存在"})
		return
	}
	payload := draftPayload(draft, nil, nil)
	rows, _ := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT stage, reviewer_id, verdict, COALESCE(CAST(issue_list AS TEXT),''), edited_by_teacher, reviewed_content_hash, reviewed_at
		 FROM ai_content_review WHERE draft_id = $1 ORDER BY id`, id)
	reviews := make([]gin.H, 0)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var stage, verdict, issues, hash, at string
			var reviewerID int64
			var edited bool
			if rows.Scan(&stage, &reviewerID, &verdict, &issues, &edited, &hash, &at) == nil {
				reviews = append(reviews, gin.H{"stage": stage, "reviewer_id": reviewerID, "verdict": verdict,
					"issue_list": issues, "edited_by_teacher": edited, "reviewed_content_hash": hash, "reviewed_at": at})
			}
		}
	}
	payload["reviews"] = reviews
	c.JSON(http.StatusOK, envelope{Code: 0, Data: payload})
}

func (s *Server) adminReviewAIDraft(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Verdict   string   `json:"verdict"`
		IssueList []string `json:"issue_list"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || (input.Verdict != "approved" && input.Verdict != "rejected") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "verdict 只接受 approved / rejected"})
		return
	}
	if input.Verdict == "rejected" && len(input.IssueList) == 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "驳回必须填写问题清单"})
		return
	}
	draft, err := scanAIDraft(s.dbConn.QueryRowContext(c.Request.Context(), aiDraftQuery+` WHERE id = $1`, id))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "草稿不存在"})
		return
	}
	if draft.Status != "pending_admin" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "草稿不在待二审状态"})
		return
	}
	lectures, err := aicontent.ParseLectures(draft.LecturesJSON, draft.LectureCount)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "草稿内容损坏"})
		return
	}
	hash := aicontent.Hash(lectures)
	var teacherApproved int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM ai_content_review WHERE draft_id=$1 AND stage='teacher' AND verdict='approved' AND reviewed_content_hash=$2`,
		id, hash).Scan(&teacherApproved)
	if teacherApproved == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "缺少当前版本的一审通过记录"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	issues, _ := json.Marshal(input.IssueList)
	nextStatus := "admin_approved"
	if input.Verdict == "rejected" {
		nextStatus = "admin_rejected"
	}
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "二审提交失败"})
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(c.Request.Context(),
		`INSERT INTO ai_content_review(draft_id, stage, reviewer_id, verdict, issue_list,
		   edited_by_teacher, reviewed_content_hash, reviewed_at) VALUES($1,'admin',$2,$3,$4,false,$5,$6)`,
		id, admin.ID, input.Verdict, string(issues), hash, time.Now().UTC().Format(time.RFC3339)); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "该版本已有二审结论"})
		return
	}
	if _, err := tx.ExecContext(c.Request.Context(),
		`UPDATE ai_content_draft SET status=$1, updated_at=$2 WHERE id=$3`,
		nextStatus, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "二审提交失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "二审提交失败"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "ai.draft_admin_review",
		TargetType: "ai_content_draft", TargetID: strconv.FormatInt(id, 10), After: nextStatus, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": nextStatus}})
}

func (s *Server) adminPublishAIDraft(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, err := scanAIDraft(s.dbConn.QueryRowContext(c.Request.Context(), aiDraftQuery+` WHERE id = $1`, id))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "草稿不存在"})
		return
	}
	if draft.Status != "admin_approved" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "草稿未通过双审，不能上架"})
		return
	}
	lectures, err := aicontent.ParseLectures(draft.LecturesJSON, draft.LectureCount)
	if err != nil || (draft.DraftScope == "chapter" && len(lectures) != draft.LectureCount) {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "章级草稿必须一次性插入全部讲次"})
		return
	}
	hash := aicontent.Hash(lectures)
	var approved int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(DISTINCT stage) FROM ai_content_review WHERE draft_id=$1 AND verdict='approved' AND reviewed_content_hash=$2`,
		id, hash).Scan(&approved)
	if approved < 2 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "双审记录不完整，不能上架"})
		return
	}
	if s.cfg == nil || s.cfg.ContentDir == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容目录未配置"})
		return
	}
	var courseID, maxNo int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, COALESCE((SELECT MAX(no) FROM lecture WHERE course_id = course.id),0) FROM course WHERE course_code=$1 AND deleted_at IS NULL`,
		draft.CourseCode).Scan(&courseID, &maxNo); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
		return
	}
	defer tx.Rollback()
	written := make([]string, 0)
	for idx, lecture := range lectures {
		no := int(maxNo) + idx + 1
		storageKey := fmt.Sprintf("courses/%s/lectures/%d/markdown/ai-draft-%d-%d.md", draft.CourseCode, no, id, idx)
		path := filepath.Join(s.cfg.ContentDir, filepath.FromSlash(storageKey))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			cleanupFiles(written)
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
			return
		}
		body := "# " + lecture.Title + "\n\n" + lecture.ContentMD
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			cleanupFiles(written)
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
			return
		}
		written = append(written, path)
		var lectureID int64
		if err := tx.QueryRowContext(c.Request.Context(),
			`INSERT INTO lecture(course_id,no,title,is_preview,origin,in_exam_scope) VALUES($1,$2,$3,false,'ai_assisted',false) RETURNING id`,
			courseID, no, lecture.Title).Scan(&lectureID); err != nil {
			cleanupFiles(written)
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
			return
		}
		if _, err := tx.ExecContext(c.Request.Context(),
			`INSERT INTO asset(lecture_id,kind,title,storage_key,size_bytes,aigc_label,aigc_meta_written)
			 VALUES($1,'markdown','AI 辅助讲义',$2,$3,'assisted',true)`,
			lectureID, storageKey, len(body)); err != nil {
			cleanupFiles(written)
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
			return
		}
	}
	var newVersion int
	if err := tx.QueryRowContext(c.Request.Context(),
		`UPDATE course SET lecture_count=(SELECT COUNT(*) FROM lecture WHERE course_id=$1),
		        content_version=content_version+1, updated_at=$2 WHERE id=$1 RETURNING content_version`,
		courseID, now).Scan(&newVersion); err != nil {
		cleanupFiles(written)
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
		return
	}
	var revisionNo int
	if err := tx.QueryRowContext(c.Request.Context(),
		`SELECT COALESCE(MAX(revision_no),0)+1 FROM course_revision WHERE course_code=$1`, draft.CourseCode).Scan(&revisionNo); err != nil {
		cleanupFiles(written)
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
		return
	}
	if _, err := tx.ExecContext(c.Request.Context(),
		`INSERT INTO course_revision(course_code, revision_no, content_version, plan_id, changed_digest, published_at)
		 VALUES($1,$2,$3,NULL,$4,$5)`,
		draft.CourseCode, revisionNo, newVersion, fmt.Sprintf("AI 增补 %d 讲（草稿 #%d）", len(lectures), id), now); err != nil {
		cleanupFiles(written)
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
		return
	}
	if _, err := tx.ExecContext(c.Request.Context(),
		`UPDATE ai_content_draft SET status='published', aigc_meta_written=true, content_version=$2, updated_at=$3 WHERE id=$1`,
		id, newVersion, now); err != nil {
		cleanupFiles(written)
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		cleanupFiles(written)
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上架失败"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "ai.draft_publish",
		TargetType: "ai_content_draft", TargetID: strconv.FormatInt(id, 10),
		After: `{"lectures":` + strconv.Itoa(len(lectures)) + `}`, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "published", "lectures": len(lectures)}})
}

func cleanupFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

// teacherAIDraftEffect GET /teacher/ai-drafts/:id/effect —— AI 增补讲次上架前后 30 天效果对比。
func (s *Server) teacherAIDraftEffect(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, ok := s.aiDraftOwned(c, id)
	if !ok {
		return
	}
	s.writeAIDraftEffect(c, draft)
}

// adminAIDraftEffect GET /admin/ai-drafts/:id/effect —— 管理员查看 AI 增补效果。
func (s *Server) adminAIDraftEffect(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	draft, err := scanAIDraft(s.dbConn.QueryRowContext(c.Request.Context(), aiDraftQuery+` WHERE id = $1`, id))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "草稿不存在"})
		return
	}
	s.writeAIDraftEffect(c, draft)
}

func (s *Server) writeAIDraftEffect(c *gin.Context, draft aiDraft) {
	if draft.Status != "published" || !draft.ContentVersion.Valid {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "草稿尚未上架，无法对比效果"})
		return
	}
	var publishedAt string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT CAST(published_at AS TEXT) FROM course_revision WHERE course_code=$1 AND content_version=$2
		 ORDER BY revision_no DESC LIMIT 1`, draft.CourseCode, draft.ContentVersion.Int64).Scan(&publishedAt); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "版本记录不存在"})
		return
	}
	at, err := time.Parse(time.RFC3339, publishedAt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "版本时间异常"})
		return
	}
	before := s.statsWindow(c, draft.CourseCode, at.AddDate(0, 0, -30).Format("2006-01-02"), at.Format("2006-01-02"))
	after := s.statsWindow(c, draft.CourseCode, at.Format("2006-01-02"), at.AddDate(0, 0, 30).Format("2006-01-02"))
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"draft_id": draft.ID, "course_code": draft.CourseCode,
		"content_version": draft.ContentVersion.Int64, "published_at": publishedAt,
		"before": before, "after": after,
		"note": "AI 增补不回溯：已学进度与已发证书不受影响；对比仅作改版参考",
	}})
}
