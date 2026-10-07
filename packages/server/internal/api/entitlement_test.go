package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestEntitlementsRouteReturnsOnlyAuthenticatedStudentAccess(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO major(name) VALUES('样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents) VALUES('C1','platform','样例课程','main',1,'published',1,1000)`); err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO orders(order_no,student_id,course_id,course_name_snapshot,price_snapshot_cents,amount_cents,status,expired_at,paid_at) VALUES('O1',1,1,'样例课程',1000,1000,'paid','2099-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO entitlement(student_id,course_id,source,order_id,granted_at) VALUES(1,1,'order',1,'2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()
	response := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/entitlements", ``, session.Token)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"course_id":1`, `"status":"active"`) {
		t.Fatalf("权益接口错误: %d %s", response.Code, response.Body.String())
	}
}
