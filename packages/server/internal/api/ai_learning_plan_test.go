package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMyLearningPlan(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("AI 请求异常: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "下一步：完成第 2 讲；复习一致性与可用性；来源：第 1 讲。"}}},
			"usage":   map[string]int{"prompt_tokens": 20, "completion_tokens": 15},
		})
	}))
	defer fake.Close()
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	dir := t.TempDir()
	server.cfg.ContentDir = dir
	server.cfg.LLMBaseURL = fake.URL
	server.cfg.DeepSeekKey = "key"
	server.cfg.LLMModel = "plan-model"
	server.cfg.LLMFilingNo = "FIL-PLAN"
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("# 分布式\n\n一致性与可用性是核心。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义','kb/l1.md')`); err != nil {
		t.Fatal(err)
	}
	session, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/learning-plan?course_code=C1", "", session.Token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"plan"`, `"model":"plan-model"`, `"filing_no":"FIL-PLAN"`, `"citations"`) {
		t.Fatalf("学习路径生成失败: %d %s", got.Code, got.Body.String())
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='ai.learning_plan'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("学习路径必须写审计: %d %v", audits, err)
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/learning-plan?course_code=NOPE", "", session.Token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404，实际 %d", got.Code)
	}
	server.cfg.LLMFilingNo = ""
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/learning-plan?course_code=C1", "", session.Token); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺备案号应 fail-closed，实际 %d", got.Code)
	}
}
