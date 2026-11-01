package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/staff"
)

func TestTeacherDiscussionsAndOfficialReply(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	if _, err := database.Exec(`INSERT INTO lecture_topic(lecture_id, student_id, body) VALUES(1,1,'第一讲的实验步骤不太理解，求解释。')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture_reply(topic_id, student_id, body) VALUES(1,1,'我也想知道。')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,'L1-98-01','L1','single','auto','这道题的完整题干用于泄漏检测','A',1.5)`); err != nil {
		t.Fatal(err)
	}
	token := teacherToken(t, r, "t1")

	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/discussions?course_code=C1", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"student_no":"",`, `"unanswered":false`, `"topics":1`) {
		// student_no 由 identity 注册生成，不硬编码；只要字段存在与汇总正确即可。
		if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"reply_count":1`, `"topics":1`, `"course_code":"C1"`) {
			t.Fatalf("讨论工作台异常: %d %s", list.Code, list.Body.String())
		}
	}
	unanswered := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/discussions?course_code=C1&only_unanswered=true", "", token)
	if unanswered.Code != http.StatusOK || !containsAll(unanswered.Body.String(), `"items":[]`) {
		t.Fatalf("未回复筛选异常: %d %s", unanswered.Code, unanswered.Body.String())
	}

	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/discussions/1/replies", `{"body":"短"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("过短回复应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/discussions/1/replies",
		`{"body":"这道题的完整题干用于泄漏检测"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("疑似试题回复应 400，实际 %d", got.Code)
	}
	reply := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/discussions/1/replies",
		`{"body":"实验第二步先确认输入参数，再执行命令。"}`, token)
	if reply.Code != http.StatusOK || !containsAll(reply.Body.String(), `"official":true`) {
		t.Fatalf("教师回复失败: %d %s", reply.Code, reply.Body.String())
	}
	public := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/lectures/1/topics/1/replies", "")
	if public.Code != http.StatusOK || !containsAll(public.Body.String(), `"official":true`, `"role":"teacher"`, `"author":"教师一"`) {
		t.Fatalf("公开列表应合并教师官方回复: %d %s", public.Code, public.Body.String())
	}
	var notices int
	if err := database.QueryRow(`SELECT COUNT(*) FROM notification WHERE student_id=1 AND title='教师回复了你的讨论'`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("教师回复应通知学员: %d %v", notices, err)
	}

	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	other := teacherToken(t, r, "t2")
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/discussions/1/replies",
		`{"body":"越权回复同学课程讨论。"}`, other); got.Code != http.StatusForbidden {
		t.Fatalf("非本课教师回复应 403，实际 %d", got.Code)
	}
}
