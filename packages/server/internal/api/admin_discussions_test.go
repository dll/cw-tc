package api

import (
	"net/http"
	"testing"
)

func TestAdminDiscussionModeration(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO lecture_topic(id,lecture_id,student_id,body) VALUES(1,1,1,'需要老师解释一下这一讲的实验步骤。')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture_reply(id,topic_id,student_id,body) VALUES(1,1,1,'我也想知道。')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture_reply_staff(id,topic_id,staff_id,body) VALUES(1,1,2,'教师回复内容。')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/discussions", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"student_name":"样例学员"`, `"reply_count":2`) {
		t.Fatalf("讨论治理列表异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ugc/topic/1/remove",
		`{"reason_kind":"差评","confirm":true}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("差评原因必须被拒: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ugc/topic/1/remove",
		`{"reason_kind":"abuse","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("下架帖子失败: %d %s", got.Code, got.Body.String())
	}
	if public := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/lectures/1/topics", ""); public.Code != http.StatusOK || containsString(public.Body.String(), "需要老师解释") {
		t.Fatalf("下架帖子不应公开可见: %d %s", public.Code, public.Body.String())
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/lectures/1/topics/1/replies", ""); got.Code != http.StatusNotFound {
		t.Fatalf("下架帖子的回复应不可读: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ugc/topic/1/restore",
		`{"note":"误判恢复","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("恢复帖子失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ugc/reply/1/remove",
		`{"reason_kind":"advertising","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("下架学员回复失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ugc/teacher_reply/1/remove",
		`{"reason_kind":"illegal","confirm":true}`, token); got.Code != http.StatusOK {
		t.Fatalf("下架教师回复失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/ugc/bad/1/remove",
		`{"reason_kind":"abuse","confirm":true}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法类型应 400: %d", got.Code)
	}
	var notices int
	if err := database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1 AND title='讨论内容已下架'`).Scan(&notices); err != nil || notices == 0 {
		t.Fatalf("下架应通知作者: %d %v", notices, err)
	}
}
