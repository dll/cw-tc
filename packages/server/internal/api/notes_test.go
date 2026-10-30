package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestNotesSuggestionsAndExport(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()

	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/notes", `{"course_code":"C1","lecture_no":1,"title":"要点","body":"分层设计是核心。"}`, token)
	if created.Code != http.StatusOK || !strings.Contains(created.Body.String(), `"id"`) {
		t.Fatalf("新建笔记失败: %d %s", created.Code, created.Body.String())
	}
	list := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/notes?course_code=C1", ``, token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"title":"要点"`, `"body":"分层设计是核心。"`) {
		t.Fatalf("笔记列表异常: %d %s", list.Code, list.Body.String())
	}
	noteID := "1"
	if got := performJSONWithCookie(t, r, http.MethodPatch, "/api/v1/notes/"+noteID, `{"title":"更新","body":"更新后的笔记内容。"}`, token); got.Code != http.StatusOK {
		t.Fatalf("更新笔记失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/suggestions", `{"body":"短"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("过短建议应 400: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/suggestions", `{"body":"建议增加更多练习题和案例讲解。"}`, token); got.Code != http.StatusOK {
		t.Fatalf("提交建议失败: %d %s", got.Code, got.Body.String())
	}
	sugs := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/suggestions", ``, token)
	if sugs.Code != http.StatusOK || !containsAll(sugs.Body.String(), `"status":"pending"`, `"建议增加更多练习题和案例讲解。"`) {
		t.Fatalf("建议列表异常: %d %s", sugs.Code, sugs.Body.String())
	}
	exported := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/export", ``, token)
	if exported.Code != http.StatusOK || !containsAll(exported.Body.String(), `"phone":"138****5678"`, `"notes"`, `"scores"`, `"certificates"`) {
		t.Fatalf("数据导出异常: %d %s", exported.Code, exported.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodDelete, "/api/v1/notes/"+noteID, ``, token); got.Code != http.StatusOK {
		t.Fatalf("删除笔记失败: %d", got.Code)
	}
	if strings.Contains(exported.Body.String(), `"id_card"`) {
		t.Fatalf("导出不应包含身份证字段: %s", exported.Body.String())
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/notes", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录笔记应 401: %d", got.Code)
	}
}

func TestPushNoticeDedup(t *testing.T) {
	s, _, cleanup := orderLifecycleServer(t)
	defer cleanup()
	var sid int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&sid)
	s.pushNotice(context.Background(), sid, "system", "标题", "正文", "/x")
	s.pushNotice(context.Background(), sid, "system", "标题", "正文", "/x")
	var n int
	_ = s.dbConn.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=$1 AND title='标题'`, sid).Scan(&n)
	if n != 1 {
		t.Fatalf("重复通知应去重: %d", n)
	}
}
