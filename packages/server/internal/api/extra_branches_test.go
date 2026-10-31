package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestNoteAndSuggestionErrorBranches(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	if got := performJSONWithCookie(t, r, http.MethodPatch, "/api/v1/notes/999", `{"body":"内容"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在笔记应 404，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPatch, "/api/v1/notes/1", `{"body":""}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("空笔记正文应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/suggestions", `{"body":"`+strings.Repeat("长", 1001)+`"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("超长建议应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/suggestions", `{"course_code":"NOPE","body":"这条建议关联的课程并不存在。"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/suggestions", `not-json`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("坏 JSON 应 400，实际 %d", got.Code)
	}
}

func TestNotificationReadErrorBranches(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/notifications/bad/read", ``, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法通知号应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/notifications/999/read", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在通知应 404，实际 %d", got.Code)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO notification(student_id, type, title, body, link) VALUES(1,'system','标题','正文','/x')`); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/notifications/1/read", ``, token); got.Code != http.StatusOK {
		t.Fatalf("标记已读失败: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/notifications/read", ``, token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"updated"`) {
		t.Fatalf("全部已读失败: %d %s", got.Code, got.Body.String())
	}
}

func TestAdminSuggestionReplyValidation(t *testing.T) {
	server, database, superToken, _ := statsServer(t)
	defer database.Close()
	r := server.Router()
	if _, err := database.Exec(`INSERT INTO suggestion(student_id, body, status, created_at, updated_at) VALUES(1,'希望增加故障演练题。','pending','2026-10-01T00:00:00Z','2026-10-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"status":"bad","reply":"x"}`,
		`{"status":"accepted","reply":""}`,
		`not-json`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/suggestions/1/reply", body, superToken); got.Code != http.StatusBadRequest {
			t.Fatalf("非法回复应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/suggestions/999/reply", `{"status":"accepted","reply":"已排期"}`, superToken); got.Code != http.StatusNotFound {
		t.Fatalf("不存在建议应 404，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/suggestions/1/reply", `{"status":"accepted","reply":"已在 v2 版本改进"}`, superToken); got.Code != http.StatusOK {
		t.Fatalf("回复建议失败: %d %s", got.Code, got.Body.String())
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/suggestions?status=accepted", "", superToken)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"status":"accepted"`) {
		t.Fatalf("建议列表异常: %d %s", list.Code, list.Body.String())
	}
}

func TestLectureDiscussionErrorAndLikeBranches(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	if _, err := s.dbConn.Exec(`INSERT INTO lecture(id, course_id, no, title) VALUES(1,1,1,'第一讲')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,'L1-99-01','L1','single','auto','这道题的完整题干用于泄漏检测','A',1.5)`); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/courses/C1/lectures/1/topics"

	if got := performJSONWithCookie(t, r, http.MethodPost, base, `{"body":"短"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("过短讨论应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base, `{"body":"请联系我 13812345678 一起交流"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("含手机号讨论应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base, `{"body":"这道题的完整题干用于泄漏检测"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("疑似试题应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base, `{"body":"这道题的第二讲讲义应该怎么理解？"}`, token); got.Code != http.StatusOK {
		t.Fatalf("正常讨论应成功: %d %s", got.Code, got.Body.String())
	}
	list := performJSON(t, r, http.MethodGet, base, "")
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"author":"学员1"`) {
		t.Fatalf("讨论列表异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, base+"/bad/replies", ``, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法帖子号应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, base+"/999/replies", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在帖子应 404，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base+"/1/replies", `{"body":""}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("空回复应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base+"/1/replies", `{"body":"我也想知道这一讲的实验步骤。"}`, token); got.Code != http.StatusOK {
		t.Fatalf("正常回复应成功: %d %s", got.Code, got.Body.String())
	}
	replies := performJSONWithCookie(t, r, http.MethodGet, base+"/1/replies", ``, token)
	if replies.Code != http.StatusOK || !containsAll(replies.Body.String(), `"items"`) {
		t.Fatalf("回复列表异常: %d %s", replies.Code, replies.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base+"/1/like", ``, token); got.Code != http.StatusOK {
		t.Fatalf("点赞失败: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base+"/1/like", ``, token); got.Code != http.StatusOK {
		t.Fatalf("重复点赞应幂等: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodDelete, base+"/1/like", ``, token); got.Code != http.StatusOK {
		t.Fatalf("取消点赞失败: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, base+"/999/like", ``, token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在帖子点赞应 404，实际 %d", got.Code)
	}
}
