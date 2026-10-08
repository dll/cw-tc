package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminListUsersAndReveal(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "admin", "管理员", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S1','学员','13812345678',X'01',X'02','v1')`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 列表脱敏
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/users", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"phone":"138****5678"`) {
		t.Fatalf("用户列表应脱敏 = %d %s", list.Code, list.Body.String())
	}
	// 看全号需事由
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reveal", `{}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("无事故应 400，实际 %d", got.Code)
	}
	// 看全号（事由）→ 返回全号 + 写审计
	reveal := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/1/reveal", `{"reason":"投诉处理"}`, token)
	if reveal.Code != http.StatusOK || !containsAll(reveal.Body.String(), `"phone":"13812345678"`) {
		t.Fatalf("看全号失败 = %d %s", reveal.Code, reveal.Body.String())
	}
	var n int
	_ = database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='user.reveal_phone'`).Scan(&n)
	if n != 1 {
		t.Fatalf("看全号应写审计 1 条，实际 %d", n)
	}
	// 不存在的用户 → 404
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/users/999/reveal", `{"reason":"x"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("看全号不存在用户应 404，实际 %d", got.Code)
	}
}
