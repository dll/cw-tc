package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminLoginAndLedgerRoutes(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	// 建运营管理员
	if _, err := staff.Create(context.Background(), database, "admin", "管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	// 台账数据
	if _, err = database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S1','学员','13812345678',X'01',X'02','v1')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(2,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程','main',1,'published',2)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO orders(id, order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, channel, expired_at) VALUES(1,'O1',1,1,'课程',100,100,'paid','wechat','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO refund(id, refund_no, order_id, kind, amount_cents, reason, initiator, status) VALUES(1,'R1',1,'normal',100,'退款','student','success')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO certificate(id, cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status) VALUES(1,'CW-CN-A202600001',1,1,'L1',88,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','valid')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	// 登录
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("管理员登录 = %d %s", login.Code, login.Body.String())
	}
	token := extractJSONField(t, login.Body.String(), "token")

	// 无 token 访问 → 401
	if got := performJSON(t, r, http.MethodGet, "/api/v1/admin/orders", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 访问台账应 401，实际 %d", got.Code)
	}
	// 带 token 访问 → 200 + 脱敏手机号
	rec := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/orders", "", token)
	if rec.Code != http.StatusOK || !containsAll(rec.Body.String(), `"items"`, `"138****5678"`) {
		t.Fatalf("订单台账 = %d %s", rec.Code, rec.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/refunds", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"items"`) {
		t.Fatalf("退款台账 = %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/certs", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"items"`) {
		t.Fatalf("证书台账 = %d %s", got.Code, got.Body.String())
	}
	// 登录错误路径：坏 JSON → 400，错密码 → 401
	if got := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{`); got.Code != http.StatusBadRequest {
		t.Fatalf("坏 JSON 应 400，实际 %d", got.Code)
	}
	if got := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"wrong"}`); got.Code != http.StatusUnauthorized {
		t.Fatalf("错密码应 401，实际 %d", got.Code)
	}
}
