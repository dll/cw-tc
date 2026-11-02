package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminGenerateAIInsight(t *testing.T) {
	invalidEvidence := false
	invalidJSON := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evidence := `["stats.learning","comment.1","comments.count","topics.unanswered","plan.1.after.learning"]`
		if invalidEvidence {
			evidence = `["made.up"]`
		}
		content := `{"summary":"学习人数偏低，建议补充示例。","signals":[{"key":"stats.learning","text":"在学人数 1"}],"feedback_clusters":[{"label":"缺少例子","count":1,"comment_ids":[1]}],"suggestions":[{"problem":"例子不足","measure":"补充案例","priority":"P1","target_metric":"learning +20%","evidence":` + evidence + `}],"unknowns":["样本量小"]}`
		if invalidJSON {
			content = "not-json"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
			"usage":   map[string]int{"prompt_tokens": 30, "completion_tokens": 20},
		})
	}))
	defer fake.Close()
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	server.cfg.LLMBaseURL = fake.URL
	server.cfg.DeepSeekKey = "k"
	server.cfg.LLMModel = "insight-model"
	server.cfg.LLMFilingNo = "FIL-INSIGHT"
	if _, err := database.Exec(`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at)
		VALUES('C1','2026-10-31',10,2,1,0,0,0,1,1,1,0,'2026-10-31T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO revision_plan(id,course_code,source,problem,evidence,measure,target_metric,status,created_at,updated_at)
		VALUES(1,'C1','ai','例子不足','周报建议','补充案例','learning +20%','done','2026-10-01T00:00:00Z','2026-10-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course_revision(id,course_code,revision_no,content_version,plan_id,changed_digest,published_at)
		VALUES(1,'C1',1,2,1,'补充案例','2026-10-31T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at) VALUES('C1','2026-10-20',10,1,5,2,1,1,1,1,5,0,'2026-10-20T00:00:00Z')`,
		`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at) VALUES('C1','2026-11-05',20,2,8,4,3,2,2,2,8,0,'2026-11-05T00:00:00Z')`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`INSERT INTO course_comment(id,course_code,student_id,rating,body,nickname_shown,status)
		VALUES(1,'C1',1,3,'希望增加更多实例和练习，特别是分布式一致性的实际案例。','学员***','visible')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/ai-insights/generate", `{}`, token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"suggestions"`, `"evidence_map"`, `"filing_no":"FIL-INSIGHT"`, `"stats.learning"`, `"plan.1.after.learning"`) {
		t.Fatalf("AI 运营报告生成失败: %d %s", got.Code, got.Body.String())
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/C1/ai-insights", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"summary"`, `"suggestions"`) {
		t.Fatalf("报告列表异常: %d %s", list.Code, list.Body.String())
	}
	detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/C1/ai-insights/1", "", token)
	if detail.Code != http.StatusOK || !containsAll(detail.Body.String(), `"feedback_clusters"`, `"unknowns"`) {
		t.Fatalf("报告详情异常: %d %s", detail.Code, detail.Body.String())
	}
	invalidEvidence = true
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/ai-insights/generate", `{}`, token); got.Code != http.StatusBadGateway || !containsString(got.Body.String(), "不存在的证据") {
		t.Fatalf("不可溯源建议必须拒绝: %d %s", got.Code, got.Body.String())
	}
	invalidEvidence = false
	invalidJSON = true
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/ai-insights/generate", `{}`, token); got.Code != http.StatusBadGateway {
		t.Fatalf("非法报告 JSON 应 502，实际 %d", got.Code)
	}
	invalidJSON = false
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/C1/ai-insights/999", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在报告应 404，实际 %d", got.Code)
	}
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id) VALUES('C2','platform','无统计课程','main',1,'published',2)`); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C2/ai-insights/generate", `{}`, token); got.Code != http.StatusConflict {
		t.Fatalf("无统计课程应拒绝生成: %d", got.Code)
	}
	server.cfg.LLMFilingNo = ""
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/courses/C1/ai-insights/generate", `{}`, token); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺备案号应 fail-closed: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/NOPE/ai-insights", "", token); got.Code != http.StatusOK {
		t.Fatalf("空报告列表应返回 200: %d", got.Code)
	}
}

func containsString(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
