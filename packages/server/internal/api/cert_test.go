package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestListMyCertsRoute(t *testing.T) {
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
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO certificate(id, cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status) VALUES(1,'CW-CN-A202600001',1,1,'L1',88,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','valid')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO certificate(id, cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status, revoked_reason, revoked_at) VALUES(2,'CW-CN-A202600002',1,1,'L2',70,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','revoked','order_refunded','2026-10-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/certs", "", session.Token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), `"cert_no":"CW-CN-A202600001"`, `"course_name":"课程"`,
		`"display_status":"valid"`, `"can_download":true`,
		`"cert_no":"CW-CN-A202600002"`, `"display_status":"revoked"`, `"can_download":false`, `"revoked_reason":"order_refunded"`) {
		t.Fatalf("我的证书 = %d %s", got.Code, got.Body.String())
	}
}

func TestDownloadMyCertRoute(t *testing.T) {
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
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id, cert_names) VALUES(1,'C1','platform','课程','main',1,'published',1,'{"L1":"初级"}')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO certificate(id, cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status) VALUES(1,'CW-CN-A202600001',1,1,'L1',88,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','valid')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/certs/CW-CN-A202600001", "", session.Token)
	if got.Code != http.StatusOK || !containsAll(got.Body.String(), "畅网智能科技有限公司", "样例学员", "CW-CN-A202600001", "初级") {
		t.Fatalf("证书下载页 = %d %s", got.Code, got.Body.String())
	}
	if _, err = database.Exec(`INSERT INTO certificate(id, cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status, revoked_reason, revoked_at) VALUES(2,'CW-CN-A202600002',1,1,'L1',70,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','revoked','admin','2026-10-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/certs/CW-CN-A202600002/file", "", session.Token); got.Code != http.StatusNotFound {
		t.Fatalf("已撤销证书下载应 404: %d %s", got.Code, got.Body.String())
	}
	// 归属校验：另一学员访问他人证书 → 404
	other, _ := service.Register(context.Background(), identity.RegisterInput{Name: "他人", Phone: "13812345679", IDCard: "110101199003076755", Password: "password-2"})
	otherSess, _ := service.CreateSession(context.Background(), other.ID, time.Now().UTC())
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/certs/CW-CN-A202600001", "", otherSess.Token); got.Code != http.StatusNotFound {
		t.Fatalf("他人访问证书应 404: %d", got.Code)
	}
}
