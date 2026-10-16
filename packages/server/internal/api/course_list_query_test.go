package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

// decodeItems 解析响应信封并取出 data.items（断言分页条数用）。
func decodeItems(t *testing.T, body string) []map[string]any {
	t.Helper()
	var env struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, body)
	}
	return env.Data.Items
}

// seedCourseQueryDB 建一个含三门已上架课程的小库，供筛选/分页用例复用。
func seedCourseQueryDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(name) VALUES('样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES
		('G1','platform','Go 语言入门','main',1,'published',1,1000),
		('P1','platform','Python 基础','main',1,'published',1,2000),
		('G2','platform','Go 并发进阶','aux',1,'published',1,3000)`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestPublicCourseListSearchByKeyword(t *testing.T) {
	r := NewWithDatabase(testConfig(t, true), seedCourseQueryDB(t)).Router()
	resp := performJSON(t, r, http.MethodGet, "/api/v1/courses?q=Go", ``)
	body := resp.Body.String()
	if resp.Code != http.StatusOK || !containsAll(body, `"course_code":"G1"`, `"course_code":"G2"`, `"total":2`) || containsAll(body, `"course_code":"P1"`) {
		t.Fatalf("关键词筛选错误: %d %s", resp.Code, body)
	}
}

func TestPublicCourseListFilterByLine(t *testing.T) {
	r := NewWithDatabase(testConfig(t, true), seedCourseQueryDB(t)).Router()
	resp := performJSON(t, r, http.MethodGet, "/api/v1/courses?line=aux", ``)
	body := resp.Body.String()
	if resp.Code != http.StatusOK || !containsAll(body, `"course_code":"G2"`, `"total":1`) || containsAll(body, `"course_code":"G1"`) {
		t.Fatalf("line 筛选错误: %d %s", resp.Code, body)
	}
}

func TestPublicCourseListPagination(t *testing.T) {
	r := NewWithDatabase(testConfig(t, true), seedCourseQueryDB(t)).Router()
	resp := performJSON(t, r, http.MethodGet, "/api/v1/courses?page=2&page_size=1", ``)
	body := resp.Body.String()
	// 共 3 门、每页 1 门：第 2 页应恰有 1 条，且 total 仍为 3。
	if resp.Code != http.StatusOK || !containsAll(body, `"total":3`, `"page":2`, `"page_size":1`) {
		t.Fatalf("分页错误: %d %s", resp.Code, body)
	}
	if len(decodeItems(t, body)) != 1 {
		t.Fatalf("第 2 页应恰有 1 条: %s", body)
	}
}

func TestPublicCourseListRejectsBadPaging(t *testing.T) {
	r := NewWithDatabase(testConfig(t, true), seedCourseQueryDB(t)).Router()
	cases := []struct {
		query string
		code  string
	}{
		{"page_size=101", `"code":10003`},
		{"page=0", `"code":10003`},
		{"page=abc", `"code":10002`},
		{"line=bogus", `"code":10002`},
	}
	for _, tc := range cases {
		resp := performJSON(t, r, http.MethodGet, "/api/v1/courses?"+tc.query, ``)
		if resp.Code != http.StatusBadRequest || !containsAll(resp.Body.String(), tc.code) {
			t.Fatalf("参数校验错误 %s: %d %s", tc.query, resp.Code, resp.Body.String())
		}
	}
}
