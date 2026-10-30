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
	"time"
)

func TestAskAIRequiresContextQuotaAndClosesDuringExam(t *testing.T) {
	var calls int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/chat/completions" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("AI 请求异常: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "核心概念是分层设计。来源：第1讲《讲义》"}}},
			"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 8},
		})
	}))
	defer fake.Close()

	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("# 分层设计\n系统分为接入层、服务层与数据层。"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.cfg.ContentDir = dir
	s.cfg.LLMBaseURL = fake.URL
	s.cfg.DeepSeekKey = "test-key"
	s.cfg.LLMModel = "deepseek-chat"
	s.cfg.LLMFilingNo = "TEST-FILING"
	s.cfg.AIDailyQuota = 2
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	if _, err := s.dbConn.Exec(`INSERT INTO lecture(id,course_id,no,title) VALUES(1,$1,1,'第一讲')`, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义','kb/l1.md')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES($1,$2,'free','2026-01-01T00:00:00Z')`, studentID, courseID); err != nil {
		t.Fatal(err)
	}
	body := `{"course_code":"C1","lecture_no":1,"question":"这一讲的核心概念是什么？"}`
	first := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", body, token)
	if first.Code != http.StatusOK || !containsAll(first.Body.String(), `"outcome":"answered"`, `"lecture_no":1`, `"remaining":1`, `"filing_no"`) {
		t.Fatalf("AI 回答异常: %d %s", first.Code, first.Body.String())
	}
	if calls != 1 {
		t.Fatalf("模型调用次数异常: %d", calls)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", body, token); got.Code != http.StatusOK {
		t.Fatalf("第二次提问应成功: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", body, token); got.Code != http.StatusTooManyRequests {
		t.Fatalf("额度用尽应 429: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法参数应 400: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", `{"course_code":"NOPE","lecture_no":1,"question":"x"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404: %d", got.Code)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO exam_session(student_id,course_id,level,paper,status,started_at,server_deadline_at) VALUES($1,$2,'L1','[]','ongoing',?,?)`, studentID, courseID, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", body, token); got.Code != http.StatusConflict {
		t.Fatalf("测评中应关闭 AI: %d %s", got.Code, got.Body.String())
	}
	s.cfg.LLMFilingNo = "TEST-FILING"
	if _, err := s.dbConn.Exec(`UPDATE entitlement SET revoked_at=$1 WHERE student_id=$2 AND course_id=$3`, time.Now().UTC().Format(time.RFC3339), studentID, courseID); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", body, token); got.Code != http.StatusForbidden {
		t.Fatalf("无权益应 403: %d %s", got.Code, got.Body.String())
	}
	s.cfg.LLMFilingNo = ""
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/ai/ask", body, token); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺备案号应 fail-closed: %d %s", got.Code, got.Body.String())
	}
	_ = context.Background()
}
