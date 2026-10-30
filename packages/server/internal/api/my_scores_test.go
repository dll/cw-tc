package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestMyScoresListsEveryAttempt(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	var studentID int64
	if err := s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO exam_session(id,student_id,course_id,level,paper,status,started_at,submitted_at,server_deadline_at)
		VALUES(1,$1,1,'L1','[]','graded','2026-10-01T10:00:00Z','2026-10-01T10:30:00Z','2026-10-01T11:00:00Z'),
		       (2,$1,1,'L1','[]','graded','2026-10-02T10:00:00Z','2026-10-02T10:30:00Z','2026-10-02T11:00:00Z')`, studentID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO exam_result(exam_session_id,student_id,course_id,level,auto_score,manual_score,total_score,band,is_pass,graded_at)
		VALUES(1,$1,1,'L1',80,8,88,'pass',1,'2026-10-01T10:31:00Z'),
		       (2,$1,1,'L1',40,0,40,'fail',0,'2026-10-02T10:31:00Z')`, studentID); err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/scores", ``, token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(),
		`"course_code":"C1"`, `"total_score":88`, `"band":"pass"`, `"is_pass":true`,
		`"total_score":40`, `"band":"fail"`, `"is_pass":false`) {
		t.Fatalf("成绩列表异常: %d %s", got.Code, got.Body.String())
	}
	if strings.Count(got.Body.String(), `"result_id"`) != 2 {
		t.Fatalf("历次成绩不应覆盖: %s", got.Body.String())
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/me/scores", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401: %d", got.Code)
	}
}
