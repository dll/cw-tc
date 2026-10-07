package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestPublicCourseHandlersFailClosedWhenDatabaseUnavailable(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	server := NewWithDatabase(testConfig(t, true), database)
	database.Close()
	for _, path := range []string{"/api/v1/courses", "/api/v1/courses/MISSING"} {
		response := performJSON(t, server.Router(), http.MethodGet, path, ``)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s 应返回 503，实际 %d %s", path, response.Code, response.Body.String())
		}
	}
}

func TestPaymentCallbackRejectsMalformedJSONBeforeProviderStatus(t *testing.T) {
	r := New(testConfig(t, true)).Router()
	response := performJSON(t, r, http.MethodPost, "/api/v1/payment/callback", `{invalid-json`)
	if response.Code != http.StatusBadRequest || !containsAll(response.Body.String(), `"code":10002`) {
		t.Fatalf("非法 JSON 应返回参数错误: %d %s", response.Code, response.Body.String())
	}
}
