package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestExamRouteStartAndSubmit(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'sample','样例教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, cert_pass_lines) VALUES(1,'C1','platform','样例课程','main',1,'published',1,'{"L1":1.5}')`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ qno, typ, stem, ans string }{
		{"L1-01-01", "single", "赋值是表达式吗？", "A"},
		{"L1-01-02", "judge", "指针是地址。", "对"},
	} {
		if _, err = database.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,$1,'L1',$2,'auto',$3,$4,1.5)`, q.qno, q.typ, q.stem, q.ans); err != nil {
			t.Fatal(err)
		}
	}
	// 免费权益（source=free 无 order 约束）
	if _, err = database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	// 开考（用公开标识 course_code）
	start := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams", `{"course_code":"C1","level":"L1"}`, session.Token)
	if start.Code != http.StatusOK {
		t.Fatalf("开考 = %d %s", start.Code, start.Body.String())
	}
	if !containsAll(start.Body.String(), `"session_id"`, `"paper"`) {
		t.Fatalf("开考响应缺少 session_id/paper: %s", start.Body.String())
	}
	var startResp struct {
		Data struct {
			SessionID int64 `json:"session_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &startResp); err != nil {
		t.Fatal(err)
	}
	sessionID := strconv.FormatInt(startResp.Data.SessionID, 10)

	// 交卷（两题全对 → 通过 + 自动发证）
	submit := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams/"+sessionID+"/submit", `{"answers":{"L1-01-01":"A","L1-01-02":"对"}}`, session.Token)
	if submit.Code != http.StatusOK {
		t.Fatalf("交卷 = %d %s", submit.Code, submit.Body.String())
	}
	if !containsAll(submit.Body.String(), `"is_pass":true`, `"cert_no":"CW-CN-`) {
		t.Fatalf("交卷应通过并自动发证: %s", submit.Body.String())
	}
}

func TestExamRouteRejectsWithoutEntitlement(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, _ := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	session, _ := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'sample','样例教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','样例课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	// 有题（否则 BuildPaper 先报"无题可组"，到不了权益校验）
	if _, err = database.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,'L1-01-01','L1','single','auto','题干','A',1.5)`); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	// 无权益开考 → 409
	noEnt := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams", `{"course_id":1,"level":"L1"}`, session.Token)
	if noEnt.Code != http.StatusConflict {
		t.Fatalf("无权益开考应 409，实际 %d %s", noEnt.Code, noEnt.Body.String())
	}
	// 参数错误 → 400
	bad := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams", `{"course_id":0}`, session.Token)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("缺参数应 400，实际 %d", bad.Code)
	}
	// 不存在的课程码 → 400
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams", `{"course_code":"NOPE","level":"L1"}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("不存在课程码应 400，实际 %d", got.Code)
	}
	// 不存在的会话交卷 → 409
	noSession := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams/999/submit", `{"answers":{}}`, session.Token)
	if noSession.Code != http.StatusConflict {
		t.Fatalf("不存在会话交卷应 409，实际 %d", noSession.Code)
	}
}
