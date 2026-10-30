package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestAIHelperFunctions(t *testing.T) {
	if got := questionTokens("这一讲的核心概念是什么？API 调用"); len(got) == 0 {
		t.Fatal("问题分词不应为空")
	}
	parts := splitContext("第一段\n\n第二段内容", 4)
	if len(parts) < 2 {
		t.Fatalf("分段异常: %#v", parts)
	}
	s := &Server{cfg: &config.Config{}}
	if text, cites := s.courseContext(context.Background(), 1, 1, "问题"); text != "" || len(cites) != 0 {
		t.Fatal("无内容目录应返回空上下文")
	}
}

func TestCourseContextRetrievesAcrossLectures(t *testing.T) {
	s, _, cleanup := orderLifecycleServer(t)
	defer cleanup()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "kb", "l1.md"), []byte("分层设计是核心概念。"), 0o644)
	s.cfg.ContentDir = dir
	var courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	if _, err := s.dbConn.Exec(`INSERT INTO lecture(id,course_id,no,title) VALUES(1,$1,1,'第一讲')`, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(1,'markdown','讲义','kb/l1.md')`); err != nil {
		t.Fatal(err)
	}
	text, cites := s.courseContext(context.Background(), courseID, 1, "分层设计是什么？")
	if !strings.Contains(text, "分层设计") || len(cites) != 1 || cites[0].LectureNo != 1 {
		t.Fatalf("全课程检索异常: %q %+v", text, cites)
	}
}

func TestDeepSeekErrorBranches(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()
	s := &Server{cfg: &config.Config{DeepSeekKey: "k", LLMBaseURL: bad.URL, LLMModel: "m"}}
	if _, _, _, err := s.callDeepSeek(context.Background(), "q", "ctx"); err == nil {
		t.Fatal("非 2xx 应报错")
	}
	invalid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer invalid.Close()
	s.cfg.LLMBaseURL = invalid.URL
	if _, _, _, err := s.callDeepSeek(context.Background(), "q", "ctx"); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

func TestValidCommentBodyRules(t *testing.T) {
	cases := []struct {
		body string
		ok   bool
	}{
		{"", true},
		{"太短", false},
		{"这是一条足够长的课程评价内容，用于验证规则。", true},
		{"联系我 13812345678 购买答案", false},
		{"访问 https://example.com 查看", false},
		{strings.Repeat("好", 501), false},
	}
	for _, c := range cases {
		if got := validCommentBody(c.body); got != c.ok {
			t.Fatalf("body=%q got=%v want=%v", c.body, got, c.ok)
		}
	}
}
