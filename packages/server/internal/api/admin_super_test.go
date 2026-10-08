package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminSuperOnlySettings(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "ops", "运营", staff.RoleAdminOps, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "boss", "超管", staff.RoleAdminSuper, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()

	opsLogin := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"ops","password":"Temp@123"}`)
	opsToken := extractJSONField(t, opsLogin.Body.String(), "token")
	// 运营访问 super 专属 → 403
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/settings", "", opsToken); got.Code != http.StatusForbidden {
		t.Fatalf("运营访问 settings 应 403，实际 %d", got.Code)
	}

	superLogin := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"boss","password":"Temp@123"}`)
	superToken := extractJSONField(t, superLogin.Body.String(), "token")
	// 超管访问 → 200
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/settings", "", superToken); got.Code != http.StatusOK {
		t.Fatalf("超管访问 settings 应 200，实际 %d", got.Code)
	}
}

func TestAdminChangePassword(t *testing.T) {
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
	r := NewWithDatabase(testConfig(t, true), database).Router()

	// 首登：must_change_password=true
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	if login.Code != http.StatusOK || !containsAll(login.Body.String(), `"must_change_password":true`) {
		t.Fatalf("首登应返回 must_change_password=true，实际 %d %s", login.Code, login.Body.String())
	}
	token := extractJSONField(t, login.Body.String(), "token")

	// 改密
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/change-password", `{"old_password":"Temp@123","new_password":"NewPass-1"}`, token); got.Code != http.StatusOK {
		t.Fatalf("改密 = %d %s", got.Code, got.Body.String())
	}
	// 旧密码失效
	if got := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`); got.Code != http.StatusUnauthorized {
		t.Fatalf("旧密码应 401，实际 %d", got.Code)
	}
	// 新密码登录：must_change_password=false
	login2 := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"NewPass-1"}`)
	if login2.Code != http.StatusOK || !containsAll(login2.Body.String(), `"must_change_password":false`) {
		t.Fatalf("改密后登录应 must_change=false，实际 %d %s", login2.Code, login2.Body.String())
	}
	// 旧密码错误改密 → 409
	token2 := extractJSONField(t, login2.Body.String(), "token")
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/change-password", `{"old_password":"wrong","new_password":"x"}`, token2); got.Code != http.StatusConflict {
		t.Fatalf("旧密码错误改密应 409，实际 %d", got.Code)
	}
	// 缺参数 → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/change-password", `{"old_password":""}`, token2); got.Code != http.StatusBadRequest {
		t.Fatalf("缺参数应 400，实际 %d", got.Code)
	}
}
