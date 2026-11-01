package api

import (
	"net/http"
	"testing"
	"time"
)

func TestAdminReviewQueueAndDetail(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	submitted := time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339)
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents,submitted_at,content_version)
		VALUES('RT-1','platform','待审核课程','main',1,'submitted',2,9900,$1,2)`, submitted); err != nil {
		t.Fatal(err)
	}
	var courseID int64
	if err := database.QueryRow(`SELECT id FROM course WHERE course_code='RT-1'`).Scan(&courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture(id,course_id,no,title,origin) VALUES(50,$1,1,'第一讲','platform')`, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset(lecture_id,kind,title,storage_key) VALUES(50,'markdown','讲义','courses/RT-1/lectures/1/markdown/notes.md')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO question(course_id,qno,level,type,gradability,stem,answer,score) VALUES($1,'L1-01-01','L1','single','auto','题干','A',1.5)`, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course_review(course_code,round,content_version,reviewer_id,reviewer_role,
		score_material,score_objective,score_case,score_lesson_plan,score_slides,score_lab,dimension_notes,issue_list,verdict,reviewed_at)
		VALUES('RT-1',1,2,1,'admin_ops',20,12,12,12,12,12,'', '[{"dimension":"slides","issue":"缺实验"}]','rejected','2026-10-30T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	queue := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/reviews", "", token)
	if queue.Code != http.StatusOK || !containsAll(queue.Body.String(), `"course_code":"RT-1"`, `"over_sla":true`, `"teacher_name":"教师一"`) {
		t.Fatalf("审核队列异常: %d %s", queue.Code, queue.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/reviews?limit=0", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 limit 应 400，实际 %d", got.Code)
	}
	detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/RT-1/review", "", token)
	if detail.Code != http.StatusOK || !containsAll(detail.Body.String(),
		`"lecture_count":1`, `"asset_count":1`, `"question_count":0`, `"level":"L1"`, `"verdict":"rejected"`, `"content_version":2`) {
		t.Fatalf("审核详情异常: %d %s", detail.Code, detail.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/courses/NOPE/review", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在课程应 404，实际 %d", got.Code)
	}
}
