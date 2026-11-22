package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExamTakeoverFlow(t *testing.T) {
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
	if _, err := s.dbConn.Exec(`UPDATE course SET cert_pass_lines = '{"L1":100}' WHERE id = $1`, courseID); err != nil {
		t.Fatal(err)
	}
	r := s.Router()

	start := performJSONWithCookieAndDevice(t, r, http.MethodPost, "/api/v1/exams",
		`{"course_code":"C1","level":"L1"}`, token, "dev-a")
	if start.Code != http.StatusOK {
		t.Fatalf("开考失败: %d %s", start.Code, start.Body.String())
	}
	var startResp struct {
		Data struct {
			SessionID int64 `json:"session_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &startResp); err != nil || startResp.Data.SessionID <= 0 {
		t.Fatalf("开考响应缺少 session_id: %s %v", start.Body.String(), err)
	}
	sid := strconv.FormatInt(startResp.Data.SessionID, 10)

	other := performJSONWithCookieAndDevice(t, r, http.MethodGet,
		"/api/v1/me/exams/ongoing?course_code=C1&level=L1", ``, token, "dev-b")
	if other.Code != http.StatusOK || !containsAll(other.Body.String(), `"needs_takeover":true`, `"session_id":`) ||
		strings.Contains(other.Body.String(), `"paper"`) {
		t.Fatalf("另一设备应先要求接管且不得下卷: %d %s", other.Code, other.Body.String())
	}

	take := performJSONWithCookieAndDevice(t, r, http.MethodPost,
		"/api/v1/exams/"+sid+"/takeover", `{"confirm":true}`, token, "dev-b")
	if take.Code != http.StatusOK || !containsAll(take.Body.String(), `"session_id":`, `"paper"`, `"remaining_secs":`) {
		t.Fatalf("接管失败: %d %s", take.Code, take.Body.String())
	}

	old := performJSONWithCookieAndDevice(t, r, http.MethodGet,
		"/api/v1/me/exams/ongoing?course_code=C1&level=L1", ``, token, "dev-a")
	if old.Code != http.StatusOK || !strings.Contains(old.Body.String(), `"needs_takeover":true`) {
		t.Fatalf("旧设备应要求重新接管: %d %s", old.Code, old.Body.String())
	}

	if back := performJSONWithCookieAndDevice(t, r, http.MethodPost,
		"/api/v1/exams/"+sid+"/takeover", `{"confirm":true}`, token, "dev-a"); back.Code != http.StatusOK {
		t.Fatalf("接管回应失败: %d %s", back.Code, back.Body.String())
	}
	if oldSubmit := performJSONWithCookieAndDevice(t, r, http.MethodPost,
		"/api/v1/exams/"+sid+"/submit", `{"answers":{}}`, token, "dev-b"); oldSubmit.Code != http.StatusConflict {
		t.Fatalf("旧设备交卷应拒绝: %d %s", oldSubmit.Code, oldSubmit.Body.String())
	}
	if submit := performJSONWithCookieAndDevice(t, r, http.MethodPost,
		"/api/v1/exams/"+sid+"/submit", `{"answers":{}}`, token, "dev-a"); submit.Code != http.StatusOK {
		t.Fatalf("当前设备交卷应成功: %d %s", submit.Code, submit.Body.String())
	}
}

func TestExamTakeoverErrors(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	var sid int64
	if err := s.dbConn.QueryRow(`INSERT INTO exam_session(student_id,course_id,level,paper,answers,status,started_at,server_deadline_at)
		VALUES($1,$2,'L1','[]','{}','ongoing',$3,$4) RETURNING id`,
		studentID, courseID,
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339)).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/exams/" + strconv.FormatInt(sid, 10) + "/takeover"
	r := s.Router()
	if got := performJSONWithCookieAndDevice(t, r, http.MethodPost, "/api/v1/exams/abc/takeover", `{"confirm":true}`, token, "dev-b"); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 sessionID 应 400: %d", got.Code)
	}
	if got := performJSONWithCookieAndDevice(t, r, http.MethodPost, path, `{}`, token, "dev-b"); got.Code != http.StatusBadRequest {
		t.Fatalf("未确认接管应 400: %d", got.Code)
	}
	if got := performJSONWithCookieAndDevice(t, r, http.MethodPost, path, `{"confirm":true}`, token, ""); got.Code != http.StatusBadRequest {
		t.Fatalf("缺设备标识应 400: %d", got.Code)
	}
	if got := performJSONWithCookieAndDevice(t, r, http.MethodPost, "/api/v1/exams/9999/takeover", `{"confirm":true}`, token, "dev-b"); got.Code != http.StatusNotFound {
		t.Fatalf("不存在会话应 404: %d", got.Code)
	}

	if _, err := s.dbConn.Exec(`INSERT INTO student(student_no,name,phone,agreement_versions) VALUES('S2','另一学员','13812345679','[]')`); err != nil {
		t.Fatal(err)
	}
	var otherID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student WHERE phone='13812345679'`).Scan(&otherID)
	var otherSid int64
	if err := s.dbConn.QueryRow(`INSERT INTO exam_session(student_id,course_id,level,paper,answers,status,started_at,server_deadline_at)
		VALUES($1,$2,'L1','[]','{}','ongoing',$3,$4) RETURNING id`,
		otherID, courseID,
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339)).Scan(&otherSid); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookieAndDevice(t, r, http.MethodPost,
		"/api/v1/exams/"+strconv.FormatInt(otherSid, 10)+"/takeover", `{"confirm":true}`, token, "dev-b"); got.Code != http.StatusNotFound {
		t.Fatalf("非本人会话应 404: %d %s", got.Code, got.Body.String())
	}

	if _, err := s.dbConn.Exec(`UPDATE exam_session SET status='graded' WHERE id=$1`, sid); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookieAndDevice(t, r, http.MethodPost, path, `{"confirm":true}`, token, "dev-b"); got.Code != http.StatusConflict {
		t.Fatalf("已交卷会话接管应 409: %d %s", got.Code, got.Body.String())
	}
}

func TestOngoingClaimsLegacyDevice(t *testing.T) {
	s, token, cleanup := orderLifecycleServer(t)
	defer cleanup()
	var studentID, courseID int64
	_ = s.dbConn.QueryRow(`SELECT id FROM student LIMIT 1`).Scan(&studentID)
	_ = s.dbConn.QueryRow(`SELECT id FROM course WHERE course_code='C1'`).Scan(&courseID)
	var sid int64
	if err := s.dbConn.QueryRow(`INSERT INTO exam_session(student_id,course_id,level,paper,answers,status,started_at,server_deadline_at)
		VALUES($1,$2,'L1','[]','{"1":"A"}','ongoing',$3,$4) RETURNING id`,
		studentID, courseID,
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339)).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookieAndDevice(t, s.Router(), http.MethodGet,
		"/api/v1/me/exams/ongoing?course_code=C1&level=L1", ``, token, "dev-x")
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"session_id":`, `"1":"A"`) ||
		strings.Contains(got.Body.String(), "_device_key") {
		t.Fatalf("历史会话应认领设备并隐藏保留键: %d %s", got.Code, got.Body.String())
	}
	other := performJSONWithCookieAndDevice(t, s.Router(), http.MethodGet,
		"/api/v1/me/exams/ongoing?course_code=C1&level=L1", ``, token, "dev-y")
	if other.Code != http.StatusOK || !strings.Contains(other.Body.String(), `"needs_takeover":true`) {
		t.Fatalf("认领后其他设备应要求接管: %d %s", other.Code, other.Body.String())
	}
}
