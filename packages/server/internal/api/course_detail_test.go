package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestPublicCourseDetailReturnsPublishedCourseOnly(t *testing.T) {
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
	if _, err = database.Exec(`INSERT INTO course(course_code,source,name,intro,line,major_id,status,teacher_id,price_cents) VALUES('DETAIL','platform','详情课程','课程简介','main',1,'published',1,1000),('HIDE','platform','隐藏课程','草稿','main',1,'building',1,2000)`); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()
	response := performJSON(t, r, http.MethodGet, "/api/v1/courses/DETAIL", ``)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"course_code":"DETAIL"`, `"intro":"课程简介"`) {
		t.Fatalf("课程详情错误: %d %s", response.Code, response.Body.String())
	}
	missing := performJSON(t, r, http.MethodGet, "/api/v1/courses/HIDE", ``)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("未上架课程必须隐藏: %d %s", missing.Code, missing.Body.String())
	}
}
