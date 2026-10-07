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

func TestChangePasswordRoute(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), identity.RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "old-pass-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithIdentityAndDatabase(testConfig(t, true), service, database).Router()
	wrong := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/password", `{"old_password":"bad","new_password":"new-pass-1"}`, session.Token)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("旧密码错误应返回 401: %d %s", wrong.Code, wrong.Body.String())
	}
	ok := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/password", `{"old_password":"old-pass-1","new_password":"new-pass-1"}`, session.Token)
	if ok.Code != http.StatusOK {
		t.Fatalf("改密失败: %d %s", ok.Code, ok.Body.String())
	}
	if _, err := service.Login(context.Background(), "13812345678", "new-pass-1"); err != nil {
		t.Fatalf("新密码应可登录: %v", err)
	}
}
