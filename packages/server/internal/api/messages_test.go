package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestMessagesUnreadMarkAllAndPrefs(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	var studentID int64
	if err := s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO notification(student_id,type,title,body,link)
		VALUES($1,'system','账号安全','已换绑','/settings'),($1,'broadcast','新课上架','欢迎','/courses'),($1,'order','支付成功','已开通','/me/orders')`, studentID); err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/messages", ``, token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(),
		`"unread":3`, `"broadcast_opt_out":false`, `"title":"账号安全"`, `"type":"system"`) {
		t.Fatalf("消息列表异常: %d %s", got.Code, got.Body.String())
	}
	read := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/messages/read", `{}`, token)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"updated":3`) {
		t.Fatalf("全部已读异常: %d %s", read.Code, read.Body.String())
	}
	prefs := performJSONWithCookie(t, r, http.MethodPatch, "/api/v1/me/messages/prefs", `{"broadcast_opt_out":true}`, token)
	if prefs.Code != http.StatusOK || !strings.Contains(prefs.Body.String(), `"broadcast_opt_out":true`) {
		t.Fatalf("广播退订异常: %d %s", prefs.Code, prefs.Body.String())
	}
	after := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/messages", ``, token)
	if after.Code != http.StatusOK || !containsAll(after.Body.String(), `"unread":0`, `"broadcast_opt_out":true`) {
		t.Fatalf("已读/偏好未持久化: %d %s", after.Code, after.Body.String())
	}
}
