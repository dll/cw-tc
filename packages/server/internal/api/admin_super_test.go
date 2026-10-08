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
