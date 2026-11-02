package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIDraftDoubleReviewAndPublish(t *testing.T) {
	var short bool
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := strings.Repeat("分布式系统需要一致性与可用性。", 40)
		second := strings.Repeat("可用性与一致性需要权衡取舍。", 40)
		if short {
			content = "太短"
			second = "太短"
		}
		payload := map[string]any{"lectures": []map[string]string{
			{"title": "AI 增补一", "content_md": content},
			{"title": "AI 增补二", "content_md": second},
		}}
		raw, _ := json.Marshal(payload)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": string(raw)}}},
			"usage":   map[string]int{"prompt_tokens": 20, "completion_tokens": 10},
		})
	}))
	defer fake.Close()

	server, database, adminToken, _ := adminDetailServer(t)
	defer database.Close()
	dir := t.TempDir()
	server.cfg.ContentDir = dir
	server.cfg.LLMBaseURL = fake.URL
	server.cfg.DeepSeekKey = "key"
	server.cfg.LLMModel = "gen-model"
	server.cfg.LLMFilingNo = "FIL-GEN"
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("# 分布式\n\n一致性与可用性是核心。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义','kb/l1.md')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture_topic(id,lecture_id,student_id,body) VALUES(1,1,1,'希望补充分布式一致性的例子。')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	teacher := teacherToken(t, r, "t1")
	body := `{"course_code":"C1","draft_scope":"chapter","kind":"new_lecture","trigger_kind":"teacher_mark","source_topic_ids":[1],"lecture_count":2}`
	created := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts", body, teacher)
	if created.Code != http.StatusOK || !containsAll(created.Body.String(), `"status":"pending_teacher"`, `"auto_check_passed":true`, `"aigc_labeled":true`) {
		t.Fatalf("生成草稿失败: %d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &payload)
	draftID := payload.Data.ID
	if draftID <= 0 {
		t.Fatalf("草稿 id 异常: %s", created.Body.String())
	}
	// 跳过一审直接二审必须被拒。
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(draftID)+"/review", `{"verdict":"approved"}`, adminToken); got.Code != http.StatusConflict {
		t.Fatalf("跳过一审应 409，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts/"+itoaTest(draftID)+"/review", `{"verdict":"rejected"}`, teacher); got.Code != http.StatusBadRequest {
		t.Fatalf("驳回缺问题清单应 400，实际 %d", got.Code)
	}
	first := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts/"+itoaTest(draftID)+"/review",
		`{"verdict":"approved","edited_by_teacher":false}`, teacher)
	if first.Code != http.StatusOK || !containsAll(first.Body.String(), `"status":"pending_admin"`) {
		t.Fatalf("一审通过失败: %d %s", first.Code, first.Body.String())
	}
	second := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(draftID)+"/review",
		`{"verdict":"approved"}`, adminToken)
	if second.Code != http.StatusOK || !containsAll(second.Body.String(), `"status":"admin_approved"`) {
		t.Fatalf("二审通过失败: %d %s", second.Code, second.Body.String())
	}
	published := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(draftID)+"/publish", `{}`, adminToken)
	if published.Code != http.StatusOK || !containsAll(published.Body.String(), `"status":"published"`, `"lectures":2`) {
		t.Fatalf("上架失败: %d %s", published.Code, published.Body.String())
	}
	var aiLectures, scoped int
	if err := database.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN in_exam_scope=0 THEN 1 ELSE 0 END) FROM lecture WHERE origin='ai_assisted'`).Scan(&aiLectures, &scoped); err != nil || aiLectures != 2 || scoped != 2 {
		t.Fatalf("AI 讲次落库异常: %d %d %v", aiLectures, scoped, err)
	}
	var draftStatus, assetLabel string
	if err := database.QueryRow(`SELECT status FROM ai_content_draft WHERE id=$1`, draftID).Scan(&draftStatus); err != nil || draftStatus != "published" {
		t.Fatalf("草稿状态异常: %s %v", draftStatus, err)
	}
	if err := database.QueryRow(`SELECT aigc_label FROM asset WHERE kind='markdown' ORDER BY id DESC LIMIT 1`).Scan(&assetLabel); err != nil || assetLabel != "assisted" {
		t.Fatalf("AI 资源标识异常: %s %v", assetLabel, err)
	}
	var reviews int
	if err := database.QueryRow(`SELECT COUNT(*) FROM ai_content_review WHERE draft_id=$1`, draftID).Scan(&reviews); err != nil || reviews != 2 {
		t.Fatalf("双审记录异常: %d %v", reviews, err)
	}
	var revisions int
	if err := database.QueryRow(`SELECT COUNT(*) FROM course_revision WHERE course_code='C1' AND content_version=(SELECT content_version FROM ai_content_draft WHERE id=$1)`, draftID).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatalf("AI 上架必须写课程版本记录: %d %v", revisions, err)
	}
	for _, stmt := range []string{
		`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at) VALUES('C1',date('now','-10 day'),10,1,5,2,1,1,1,1,5,0,datetime('now'))`,
		`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at) VALUES('C1',date('now','+1 day'),20,2,8,4,3,2,2,2,8,0,datetime('now'))`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	effect := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/ai-drafts/"+itoaTest(draftID)+"/effect", "", adminToken)
	if effect.Code != http.StatusOK || !containsAll(effect.Body.String(), `"before"`, `"after"`, `"learning":5`, `"learning":8`) {
		t.Fatalf("AI 增补效果对比异常: %d %s", effect.Code, effect.Body.String())
	}
	teacherEffect := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/ai-drafts/"+itoaTest(draftID)+"/effect", "", teacher)
	if teacherEffect.Code != http.StatusOK {
		t.Fatalf("教师效果对比异常: %d", teacherEffect.Code)
	}

	// 体检未通过不得进入一审。
	short = true
	if _, err := database.Exec(`INSERT INTO lecture_topic(id,lecture_id,student_id,body) VALUES(2,1,1,'希望补充例子。')`); err != nil {
		t.Fatal(err)
	}
	bad := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts",
		`{"course_code":"C1","draft_scope":"chapter","source_topic_ids":[2],"lecture_count":2}`, teacher)
	if bad.Code != http.StatusOK || !containsAll(bad.Body.String(), `"status":"drafting"`, `"auto_check_passed":false`) {
		t.Fatalf("体检失败草稿异常: %d %s", bad.Code, bad.Body.String())
	}
	var badPayload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bad.Body.Bytes(), &badPayload)
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts/"+itoaTest(badPayload.Data.ID)+"/review",
		`{"verdict":"approved"}`, teacher); got.Code != http.StatusConflict {
		t.Fatalf("未过体检不得一审，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/ai-drafts/"+itoaTest(badPayload.Data.ID)+"/effect", "", teacher); got.Code != http.StatusConflict {
		t.Fatalf("未上架草稿不能对比效果: %d", got.Code)
	}
}

func TestAIDraftBranchesAndRejections(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := strings.Repeat("基于课程资料补充的解释内容。", 40)
		payload := map[string]any{"lectures": []map[string]string{{"title": "AI 单讲", "content_md": content}}}
		raw, _ := json.Marshal(payload)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": string(raw)}}},
			"usage":   map[string]int{"prompt_tokens": 5, "completion_tokens": 5},
		})
	}))
	defer fake.Close()
	server, database, adminToken, _ := adminDetailServer(t)
	defer database.Close()
	dir := t.TempDir()
	server.cfg.ContentDir = dir
	server.cfg.LLMBaseURL = fake.URL
	server.cfg.DeepSeekKey = "k"
	server.cfg.LLMModel = "m"
	server.cfg.LLMFilingNo = "FIL"
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("分布式一致性资料"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义','kb/l1.md')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture_topic(id,lecture_id,student_id,body) VALUES(10,1,1,'希望补充例子。')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	teacher := teacherToken(t, r, "t1")
	for _, body := range []string{
		`{"course_code":"C1","draft_scope":"single","source_topic_ids":[],"lecture_count":1}`,
		`{"course_code":"C1","draft_scope":"chapter","source_topic_ids":[10],"lecture_count":6}`,
		`{"course_code":"NOPE","draft_scope":"single","source_topic_ids":[10],"lecture_count":1}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts", body, teacher); got.Code < 400 {
			t.Fatalf("非法生成应失败: %s -> %d", body, got.Code)
		}
	}
	created := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts",
		`{"course_code":"C1","draft_scope":"single","source_topic_ids":[10],"lecture_count":1}`, teacher)
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &payload)
	id := payload.Data.ID
	if created.Code != http.StatusOK || id <= 0 {
		t.Fatalf("生成失败: %d %s", created.Code, created.Body.String())
	}
	if list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/ai-drafts", "", teacher); list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"id":`) {
		t.Fatalf("教师草稿列表异常: %d", list.Code)
	}
	if detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/ai-drafts/"+itoaTest(id), "", teacher); detail.Code != http.StatusOK {
		t.Fatalf("教师草稿详情异常: %d", detail.Code)
	}
	// 编辑后重新体检仍应待一审。
	lecturesBody := `{"lectures":[{"title":"人工修订","content_md":"` + strings.Repeat("人工修订后的内容。", 40) + `"}]}`
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/teacher/ai-drafts/"+itoaTest(id), lecturesBody, teacher); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"auto_check_passed":true`) {
		t.Fatalf("编辑草稿失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts/"+itoaTest(id)+"/review",
		`{"verdict":"rejected","issue_list":["例子不足"]}`, teacher); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"teacher_rejected"`) {
		t.Fatalf("一审驳回失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(id)+"/review",
		`{"verdict":"approved"}`, adminToken); got.Code != http.StatusConflict {
		t.Fatalf("驳回后不得二审: %d", got.Code)
	}

	// 第二个草稿：二审驳回 + 未双审不得上架。
	second := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts",
		`{"course_code":"C1","draft_scope":"single","source_topic_ids":[10],"lecture_count":1}`, teacher)
	var secondPayload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(second.Body.Bytes(), &secondPayload)
	secondID := secondPayload.Data.ID
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(secondID)+"/publish", `{}`, adminToken); got.Code != http.StatusConflict {
		t.Fatalf("未双审不得上架: %d", got.Code)
	}
	_ = performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts/"+itoaTest(secondID)+"/review", `{"verdict":"approved"}`, teacher)
	if list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/ai-drafts", "", adminToken); list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"pending_admin"`) {
		t.Fatalf("管理员队列异常: %d", list.Code)
	}
	if detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/ai-drafts/"+itoaTest(secondID), "", adminToken); detail.Code != http.StatusOK {
		t.Fatalf("管理员详情异常: %d", detail.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(secondID)+"/review",
		`{"verdict":"rejected","issue_list":["合规风险"]}`, adminToken); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"admin_rejected"`) {
		t.Fatalf("二审驳回失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(secondID)+"/publish", `{}`, adminToken); got.Code != http.StatusConflict {
		t.Fatalf("二审驳回后不得上架: %d", got.Code)
	}
	server.cfg.LLMFilingNo = ""
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts",
		`{"course_code":"C1","draft_scope":"single","source_topic_ids":[10],"lecture_count":1}`, teacher); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺备案号应 fail-closed: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/ai-drafts/999", "", teacher); got.Code != http.StatusNotFound {
		t.Fatalf("不存在草稿应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/ai-drafts/999", "", adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("管理员不存在草稿应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/999/publish", `{}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("上架不存在草稿应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/ai-drafts/"+itoaTest(id)+"/review", `{"verdict":"bad"}`, teacher); got.Code != http.StatusBadRequest {
		t.Fatalf("非法一审结论应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ai-drafts/"+itoaTest(secondID)+"/review", `{"verdict":"bad"}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法二审结论应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/teacher/ai-drafts/"+itoaTest(id), `{"lectures":[]}`, teacher); got.Code != http.StatusBadRequest {
		t.Fatalf("讲次数不一致应 400: %d", got.Code)
	}
}
