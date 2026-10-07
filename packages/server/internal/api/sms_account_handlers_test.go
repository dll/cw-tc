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

func smsAccountServer(t *testing.T) (*Server, *identity.SMSService, string, func()) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
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
	sms := identity.NewSMSService()
	return NewWithIdentityAndDatabase(testConfig(t, true), service, database).WithSMS(sms), sms, session.Token, func() { database.Close() }
}

func TestSMSLoginRoute(t *testing.T) {
	s, sms, _, cleanup := smsAccountServer(t)
	defer cleanup()
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "login", "654321", now); err != nil {
		t.Fatal(err)
	}
	response := performJSON(t, s.Router(), http.MethodPost, "/api/v1/auth/sms-login", `{"phone":"13812345678","code":"654321"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("验证码登录失败: %d %s", response.Code, response.Body.String())
	}
}

func TestResetPasswordRoute(t *testing.T) {
	s, sms, _, cleanup := smsAccountServer(t)
	defer cleanup()
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "reset", "112233", now); err != nil {
		t.Fatal(err)
	}
	response := performJSON(t, s.Router(), http.MethodPost, "/api/v1/auth/reset-password", `{"phone":"13812345678","code":"112233","new_password":"new-pass-1"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("找回密码失败: %d %s", response.Code, response.Body.String())
	}
}

func TestRebindPhoneRoute(t *testing.T) {
	s, sms, token, cleanup := smsAccountServer(t)
	defer cleanup()
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "rebind_old", "111111", now); err != nil {
		t.Fatal(err)
	}
	if _, err := sms.Issue(context.Background(), "13912345678", "rebind_new", "222222", now); err != nil {
		t.Fatal(err)
	}
	response := performJSONWithCookie(t, s.Router(), http.MethodPost, "/api/v1/me/phone", `{"old_code":"111111","phone":"13912345678","new_code":"222222"}`, token)
	if response.Code != http.StatusOK {
		t.Fatalf("换绑失败: %d %s", response.Code, response.Body.String())
	}
}

func TestDeleteAccountRoute(t *testing.T) {
	s, _, token, cleanup := smsAccountServer(t)
	defer cleanup()
	response := performJSONWithCookie(t, s.Router(), http.MethodPost, "/api/v1/me/delete", `{}`, token)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"status":"anonymized"`) {
		t.Fatalf("注销失败: %d %s", response.Code, response.Body.String())
	}
}

func TestSMSRoutesFailClosedWithoutSMS(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := identity.NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	s := NewWithIdentityAndDatabase(testConfig(t, true), service, database)
	response := performJSON(t, s.Router(), http.MethodPost, "/api/v1/auth/sms-login", `{"phone":"13812345678","code":"000000"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置短信应 fail-closed: %d %s", response.Code, response.Body.String())
	}
}
