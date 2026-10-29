package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestMyCoursesAndProgress(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte(strings.Repeat("a", 32)), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(ctx, identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(ctx, student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(1,'teacher','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id,course_code,source,name,line,major_id,status,teacher_id) VALUES
		(1,'C1','platform','课程一','main',1,'published',1),
		(2,'C2','platform','课程二','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO lecture(course_id,no,title) VALUES(1,1,'第一讲'),(1,2,'第二讲')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_at) VALUES($1,1,'free','2026-01-01T00:00:00Z')`, student.ID); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()

	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/me/courses", "", session.Token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"course_code":"C1"`, `"completed_lectures":0`, `"status":"learning"`) {
		t.Fatalf("我的课程初始状态异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":1,"completed":true}`, session.Token); got.Code != http.StatusOK {
		t.Fatalf("上报进度失败: %d %s", got.Code, got.Body.String())
	}
	list = performJSONWithBearer(t, r, http.MethodGet, "/api/v1/me/courses", "", session.Token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"completed_lectures":1`, `"percent":50`, `"status":"learning"`) {
		t.Fatalf("50%% 进度异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":2,"completed":true}`, session.Token); got.Code != http.StatusOK {
		t.Fatalf("上报第二讲失败: %d %s", got.Code, got.Body.String())
	}
	list = performJSONWithBearer(t, r, http.MethodGet, "/api/v1/me/courses", "", session.Token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"percent":100`, `"status":"completed"`) {
		t.Fatalf("完成状态异常: %d %s", list.Code, list.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/me/courses/C2/progress", `{"lecture_no":1,"completed":true}`, session.Token); got.Code != http.StatusForbidden {
		t.Fatalf("未解锁课程上报应 403: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":99}`, session.Token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在讲次应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":0}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("非法讲次应 400: %d", got.Code)
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/me/courses", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401: %d", got.Code)
	}
}
