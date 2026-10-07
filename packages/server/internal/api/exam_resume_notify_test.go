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

func TestExamResumeSaveAndNotifications(t *testing.T) {
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
	if _, err = database.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,'L1-01-01','L1','single','auto','题干','A',1.5)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES(1,1,'free','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	// 开考
	start := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams", `{"course_id":1,"level":"L1"}`, session.Token)
	var startResp struct {
		Data struct {
			SessionID int64 `json:"session_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(start.Body.Bytes(), &startResp)
	sid := strconv.FormatInt(startResp.Data.SessionID, 10)

	// 按题持久化
	save := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams/"+sid+"/answers", `{"qno":"L1-01-01","answer":"A"}`, session.Token)
	if save.Code != http.StatusOK {
		t.Fatalf("保存答案 = %d %s", save.Code, save.Body.String())
	}
	// 断线重连恢复
	resume := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/exams/"+sid, "", session.Token)
	if resume.Code != http.StatusOK || !containsAll(resume.Body.String(), `"answers"`, `"remaining_secs"`, `"L1-01-01"`) {
		t.Fatalf("恢复状态失败 = %d %s", resume.Code, resume.Body.String())
	}

	// 通知：造一条广播通知
	if _, err = database.Exec(`INSERT INTO notification(student_id, type, title, body) VALUES(1,'system','标题','正文')`); err != nil {
		t.Fatal(err)
	}
	list := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/notifications?unread=true", "", session.Token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"items"`, `"标题"`) {
		t.Fatalf("通知列表失败 = %d %s", list.Code, list.Body.String())
	}
	// 标记已读（通知 id=1）
	read := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/notifications/1/read", `{}`, session.Token)
	if read.Code != http.StatusOK {
		t.Fatalf("标记已读 = %d %s", read.Code, read.Body.String())
	}
	unread := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/notifications?unread=true", "", session.Token)
	if containsAll(unread.Body.String(), `"标题"`) {
		t.Fatalf("标记已读后未读列表应不含该通知: %s", unread.Body.String())
	}

	// 错误路径：非法 sessionID / 缺 qno / 标已读不存在的通知
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/exams/abc", "", session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法 sessionID 应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/exams/"+sid+"/answers", `{"answer":"A"}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("缺 qno 应 400，实际 %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/notifications/999/read", `{}`, session.Token); got.Code != http.StatusNotFound {
		t.Fatalf("标已读不存在的通知应 404，实际 %d", got.Code)
	}
}
