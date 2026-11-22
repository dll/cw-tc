package api

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOngoingExamReturnsPaperAndAnswers(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	var studentID, courseID int64
	if err := s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID); err != nil {
		t.Fatal(err)
	}
	if err := s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID); err != nil {
		t.Fatal(err)
	}
	paper := `[{"qno":"1","type":"single","stem":"题目","options":["A","B"],"score":10}]`
	deadline := time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339)
	if _, err := s.dbConn.Exec(`INSERT INTO exam_session(student_id,course_id,level,paper,answers,status,started_at,server_deadline_at)
		VALUES($1,$2,'L1',$3,'{"1":"A"}','ongoing',?,?)`, studentID, courseID, paper, time.Now().UTC().Format(time.RFC3339), deadline); err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/exams/ongoing?course_code=C1&level=L1", ``, token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"session_id":`, `"remaining_secs":`, `"stem":"题目"`, `"1":"A"`, `"expired":false`) {
		t.Fatalf("续考状态异常: %d %s", got.Code, got.Body.String())
	}
	none := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/exams/ongoing?course_code=C1&level=L2", ``, token)
	if none.Code != http.StatusOK || !strings.Contains(none.Body.String(), `"session":null`) {
		t.Fatalf("无会话应返回 null: %d %s", none.Code, none.Body.String())
	}
}

func TestOngoingExamExpired(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	if _, err := s.dbConn.Exec(`INSERT INTO exam_session(student_id,course_id,level,paper,status,started_at,server_deadline_at) VALUES($1,$2,'L1','[]','ongoing',$3,$4)`, studentID, courseID, time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339), time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookie(t, s.Router(), http.MethodGet, "/api/v1/me/exams/ongoing?course_code=C1&level=L1", ``, token)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"session":null`) {
		t.Fatalf("过期会话应自动结束并返回 null: %d %s", got.Code, got.Body.String())
	}
	var status string
	if err := s.dbConn.QueryRow(`SELECT status FROM exam_session LIMIT 1`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status == "ongoing" {
		t.Fatal("过期会话仍为 ongoing")
	}
}

func TestStartExamAfterExpiredOngoing(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	if _, err := s.dbConn.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES($1,'L1-01-01','L1','single','auto','题目','A',10)`, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES($1,$2,'free','2026-01-01T00:00:00Z')`, studentID, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dbConn.Exec(`INSERT INTO exam_session(student_id,course_id,level,paper,status,started_at,server_deadline_at) VALUES($1,$2,'L1','[]','ongoing',$3,$4)`, studentID, courseID, time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339), time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	start := performJSONWithCookie(t, s.Router(), http.MethodPost, "/api/v1/exams", `{"course_code":"C1","level":"L1"}`, token)
	if start.Code != http.StatusOK || !strings.Contains(start.Body.String(), `"session_id"`) {
		t.Fatalf("过期会话后应能重新开考: %d %s", start.Code, start.Body.String())
	}
}
