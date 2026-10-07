package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestVerifyCertRoute(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',X'01',X'02','[]')`,
		`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`,
		`INSERT INTO major(id, name) VALUES(1,'样例专业')`,
		`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','样例课程','main',1,'published',1)`,
		`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at) VALUES('CW-CN-A202600001',1,1,'L1',80,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z')`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	r := NewWithDatabase(testConfig(t, true), database).Router()

	// 有效 → 200，姓名脱敏
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/verify/CW-CN-A202600001", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("有效证书应 200，实际 %d", w.Code)
	}
	var body struct {
		Data struct {
			State string `json:"state"`
			Name  string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.State != "valid" {
		t.Fatalf("state 应为 valid，实际 %s", body.Data.State)
	}
	if body.Data.Name != "样***" {
		t.Fatalf("姓名应脱敏为 样***，实际 %s", body.Data.Name)
	}

	// 不存在 → 404
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/verify/UNKNOWN", nil))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("不存在应 404，实际 %d", w2.Code)
	}
}
