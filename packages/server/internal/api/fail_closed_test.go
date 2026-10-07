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

// 未注入数据库时，测评/通知接口必须 fail-closed（503），不得静默空转。
func TestExamAndNotifyFailClosedWithoutDB(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, _ := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	session, _ := service.CreateSession(context.Background(), student.ID, time.Now().UTC())

	// 只注入 identity、不注入数据库
	r := NewWithIdentity(testConfig(t, true), service).Router()

	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/exams", `{"course_id":1,"level":"L1"}`},
		{http.MethodGet, "/api/v1/exams/1", ""},
		{http.MethodPost, "/api/v1/exams/1/answers", `{"qno":"Q","answer":"A"}`},
		{http.MethodGet, "/api/v1/me/notifications", ""},
	}
	for _, tc := range cases {
		got := performJSONWithCookie(t, r, tc.method, tc.path, tc.body, session.Token)
		if got.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s 应 503，实际 %d", tc.method, tc.path, got.Code)
		}
	}
}
