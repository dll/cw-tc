package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/migrations"
)

func TestTeacherCourseLifecycle(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "t1", "教师一", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	r := NewWithDatabase(testConfig(t, true), database).Router()
	login := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"t1","password":"Temp@123"}`)
	token := extractJSONField(t, login.Body.String(), "token")

	// 新建草稿
	create := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses", `{"name":"我的平台课","line":"main","intro":"简介"}`, token)
	if create.Code != http.StatusOK || !containsAll(create.Body.String(), `"course_code"`) {
		t.Fatalf("新建草稿 = %d %s", create.Code, create.Body.String())
	}
	code := extractJSONField(t, create.Body.String(), "course_code")

	// 列表包含我的课程
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/courses", "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"我的平台课"`) {
		t.Fatalf("我的课程列表 = %d %s", got.Code, got.Body.String())
	}

	// 提交送审
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/"+code+"/submit", `{}`, token); got.Code != http.StatusOK {
		t.Fatalf("提交送审 = %d %s", got.Code, got.Body.String())
	}

	// 空名 → 400
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses", `{"name":""}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("空名应 400，实际 %d", got.Code)
	}

	// 他人课程提交 → 409（归属校验，铁律 8）
	login2 := performJSON(t, r, http.MethodPost, "/api/v1/admin/login", `{"account":"t2","password":"Temp@123"}`)
	token2 := extractJSONField(t, login2.Body.String(), "token")
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/"+code+"/submit", `{}`, token2); got.Code != http.StatusConflict {
		t.Fatalf("他人课程提交应 409，实际 %d", got.Code)
	}
	// 已提交状态再提交 → 409
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/courses/"+code+"/submit", `{}`, token); got.Code != http.StatusConflict {
		t.Fatalf("重复提交应 409，实际 %d", got.Code)
	}
}
