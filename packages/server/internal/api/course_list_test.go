package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestPublicCourseListReturnsOnlyPublishedCourses(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(name) VALUES('样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES('PUB','platform','公开课程','main',1,'published',1,1000),('DRAFT','platform','草稿课程','main',1,'building',1,2000)`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()
	response := performJSON(t, r, http.MethodGet, "/api/v1/courses", ``)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"course_code":"PUB"`) || containsAll(response.Body.String(), `"course_code":"DRAFT"`) {
		t.Fatalf("课程列表错误: %d %s", response.Code, response.Body.String())
	}
}
