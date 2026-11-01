package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/staff"
)

func TestTeacherAIGradeSuggestion(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("AI 请求异常: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "建议 3 分：要点 1 答对，要点 2 缺失。"}}},
			"usage":   map[string]int{"prompt_tokens": 12, "completion_tokens": 9},
		})
	}))
	defer fake.Close()

	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO question(id,course_id,qno,level,type,gradability,stem,answer,score)
		VALUES(100,1,'L1-99-01','L1','short','manual','请说明实验步骤','先准备环境，再执行命令。',5)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE exam_session SET paper='[{"qno":"L1-99-01","score":5}]', answers='{"L1-99-01":"先准备环境"}' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	server.cfg.LLMBaseURL = fake.URL
	server.cfg.DeepSeekKey = "key"
	server.cfg.LLMModel = "grader-model"
	server.cfg.LLMFilingNo = "FIL-GRADE"
	r := server.Router()
	token := teacherToken(t, r, "t1")
	got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/1/questions/100/ai-suggest", `{}`, token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"suggestion"`, `"model":"grader-model"`, `"filing_no":"FIL-GRADE"`) {
		t.Fatalf("AI 阅卷建议失败: %d %s", got.Code, got.Body.String())
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='ai.grade_suggest'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("AI 阅卷建议必须写审计: %d %v", audits, err)
	}
	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	other := teacherToken(t, r, "t2")
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/1/questions/100/ai-suggest", `{}`, other); got.Code != http.StatusForbidden {
		t.Fatalf("非本课教师应 403，实际 %d", got.Code)
	}
	server.cfg.LLMFilingNo = ""
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/grade/1/questions/100/ai-suggest", `{}`, token); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺备案号应 fail-closed，实际 %d", got.Code)
	}
}
