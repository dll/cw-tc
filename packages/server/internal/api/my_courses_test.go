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

func TestMyCoursesProgressIsPersistentStudentScopedAndIdempotent(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte(strings.Repeat("a", 32)), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	students := make([]identity.Student, 2)
	for i, input := range []identity.RegisterInput{
		{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"},
		{Name: "样例他人", Phone: "13812345679", IDCard: "110101199003076755", Password: "password-2"},
	} {
		students[i], err = service.Register(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('t','教师','teacher','x','seed'); INSERT INTO major(id,name) VALUES(1,'专业'); INSERT INTO course(id,course_code,source,name,line,major_id,status,price_cents,free_preview_lectures,teacher_id) VALUES(1,'C1','upstream','课程','main',1,'published',1000,1,1); INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'一',0),(2,1,2,'二',0);`); err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()
	if _, err = database.Exec(`INSERT INTO entitlement(student_id,course_id,source,granted_by,grant_reason,granted_at) VALUES($1,1,'grant',1,'test',CURRENT_TIMESTAMP)`, students[0].ID); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(ctx, students[0].ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateSession(ctx, students[1].ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	if got := performJSON(t, r, http.MethodGet, "/api/v1/me/courses", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", got.Code)
	}
	listed := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/courses", "", session.Token)
	if listed.Code != http.StatusOK || !containsAll(listed.Body.String(), `"course_code":"C1"`, `"major_name":"专业"`, `"unlocked":true`, `"status":"learning"`, `"lecture_count":2`, `"learned_lectures":0`, `"progress_percent":0`, `"last_lecture_no":null`) {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}

	visit := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":2,"completed":false}`, session.Token)
	if visit.Code != http.StatusOK || !strings.Contains(visit.Body.String(), `"last_lecture_no":2`) || !strings.Contains(visit.Body.String(), `"learned_lectures":0`) {
		t.Fatalf("visit: %d %s", visit.Code, visit.Body.String())
	}
	complete := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":2,"completed":true}`, session.Token)
	if complete.Code != http.StatusOK || !strings.Contains(complete.Body.String(), `"learned_lectures":1`) {
		t.Fatalf("complete: %d %s", complete.Code, complete.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":2,"completed":false}`, session.Token); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"learned_lectures":1`) {
		t.Fatalf("completion not sticky: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/courses", "", other.Token); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"learned_lectures":0`) {
		t.Fatalf("student data leaked: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":1,"completed":false}`, session.Token); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"last_lecture_no":1`) {
		t.Fatalf("resume did not follow latest visit: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":99,"completed":false}`, session.Token); got.Code != http.StatusNotFound {
		t.Fatalf("unknown lecture status=%d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/C1/progress", `{"lecture_no":1}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("missing completed status=%d", got.Code)
	}
}

func TestMyCoursesOrderPutsUnvisitedCoursesAfterRecentLearning(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte(strings.Repeat("a", 32)), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(ctx, identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('t','教师','teacher','x','seed'); INSERT INTO course(id,course_code,source,name,line,status,price_cents) VALUES(1,'RECENT','upstream','最近学习','main','published',0),(2,'NEW','upstream','未学习','main','published',0); INSERT INTO lecture(id,course_id,no,title) VALUES(1,1,1,'一'),(2,2,1,'一'); INSERT INTO lecture_progress(student_id,lecture_id,completed_at,last_studied_at) VALUES($1,1,NULL,'2026-01-01T00:00:00Z')`, student.ID); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(ctx, student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	got := performJSONWithCookie(t, NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router(), http.MethodGet, "/api/v1/me/courses", "", session.Token)
	recent := strings.Index(got.Body.String(), `"course_code":"RECENT"`)
	unvisited := strings.Index(got.Body.String(), `"course_code":"NEW"`)
	if got.Code != http.StatusOK || recent < 0 || unvisited < 0 || recent > unvisited {
		t.Fatalf("recent course must precede unvisited course: %d %s", got.Code, got.Body.String())
	}
}

func TestMyCoursesPreviewProgressDoesNotUnlockPaidCourse(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = db.ApplyMigrations(ctx, database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte(strings.Repeat("a", 32)), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(ctx, identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('t','教师','teacher','x','seed'); INSERT INTO course(id,course_code,source,name,line,status,price_cents,free_preview_lectures) VALUES(1,'PAID','upstream','付费课','main','published',1000,1); INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'试看',1),(2,1,2,'付费',0)`); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(ctx, student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()
	got := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/PAID/progress", `{"lecture_no":1,"completed":false}`, session.Token)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"unlocked":false`) || !strings.Contains(got.Body.String(), `"status":"locked"`) {
		t.Fatalf("preview progress must retain locked entitlement: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPut, "/api/v1/me/courses/PAID/progress", `{"lecture_no":2,"completed":false}`, session.Token); got.Code != http.StatusForbidden {
		t.Fatalf("non-preview progress must be forbidden: %d %s", got.Code, got.Body.String())
	}
}
