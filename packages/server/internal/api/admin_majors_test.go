package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestAdminMajors(t *testing.T) {
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
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"admin","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 创建（先建，保证列表能走到 Scan 分支）
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/majors", `{"name":"人工智能","sort":1}`, token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"id"`) {
		t.Fatalf("创建专业 = %d %s", got.Code, got.Body.String())
	}
	// 列表（含已建专业）
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/majors", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"name":"人工智能"`) {
		t.Fatalf("专业列表 = %d %s", got.Code, got.Body.String())
	}
	// 改名/排序
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/admin/majors/1", `{"name":"AI与大模型","sort":2,"status":"enabled"}`, token); got.Code != http.StatusOK {
		t.Fatalf("改专业 = %d %s", got.Code, got.Body.String())
	}
	// 停用
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/admin/majors/1", `{"status":"disabled"}`, token); got.Code != http.StatusOK {
		t.Fatalf("停用专业 = %d %s", got.Code, got.Body.String())
	}
	// 错误路径：空名 → 400；坏 id → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/majors", `{"name":""}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("空名应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/admin/majors/abc", `{"status":"enabled"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("坏 id 应 400，实际 %d", got.Code)
	}
}
