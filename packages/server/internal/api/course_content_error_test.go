package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestCourseContentErrorBranches(t *testing.T) {
	s, cleanup := catalogServer(t)
	defer cleanup()
	r := s.Router()
	// 未上架课程讲次返回 404
	if resp := performJSON(t, r, http.MethodGet, "/api/v1/courses/HIDDEN/lectures", ``); resp.Code != http.StatusNotFound {
		t.Fatalf("未上架课程讲次应返回 404: %d %s", resp.Code, resp.Body.String())
	}
	// 不存在的课程讲次返回 404
	if resp := performJSON(t, r, http.MethodGet, "/api/v1/courses/NOPE/lectures", ``); resp.Code != http.StatusNotFound {
		t.Fatalf("不存在课程讲次应返回 404: %d %s", resp.Code, resp.Body.String())
	}
}

func TestCourseContentFailClosedWhenDBClosed(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	s := NewWithDatabase(testConfig(t, true), database)
	database.Close()
	for _, path := range []string{"/api/v1/majors", "/api/v1/courses/AI-01/lectures"} {
		if resp := performJSON(t, s.Router(), http.MethodGet, path, ``); resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s 应返回 503: %d %s", path, resp.Code, resp.Body.String())
		}
	}
}
