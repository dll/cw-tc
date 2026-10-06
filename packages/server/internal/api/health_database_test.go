package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/platform/db"
)

func TestHealthReportsDatabaseUnavailable(t *testing.T) {
	cfg := testConfig(t, true)
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(cfg, database).Router()
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("数据库不可用时 health 状态码 = %d, body=%s", response.Code, response.Body.String())
	}
}

func TestHealthReportsDatabaseAvailable(t *testing.T) {
	cfg := testConfig(t, true)
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	r := NewWithDatabase(cfg, database).Router()
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("数据库可用时 health 状态码 = %d, body=%s", response.Code, response.Body.String())
	}
}

var _ *config.Config
