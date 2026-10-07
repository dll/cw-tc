package identity

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func accountTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	return service, database
}

func registerAccount(t *testing.T, service *Service, phone, password string) Student {
	t.Helper()
	student, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: phone, IDCard: "110101199001011237", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	return student
}

func TestLoginLocksAfterFiveFailures(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	registerAccount(t, service, "13812345678", "password-1")
	for i := 0; i < 5; i++ {
		if _, err := service.Login(context.Background(), "13812345678", "wrong-pass-1"); err != ErrInvalidCredentials {
			t.Fatalf("第 %d 次错误密码应返回 ErrInvalidCredentials: %v", i+1, err)
		}
	}
	if _, err := service.Login(context.Background(), "13812345678", "password-1"); err != ErrAccountLocked {
		t.Fatalf("5 次失败后应锁定: %v", err)
	}
}

func TestLoginResetCounterOnSuccess(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	registerAccount(t, service, "13812345678", "password-1")
	for i := 0; i < 4; i++ {
		_, _ = service.Login(context.Background(), "13812345678", "wrong-pass-1")
	}
	if _, err := service.Login(context.Background(), "13812345678", "password-1"); err != nil {
		t.Fatalf("正确密码应成功并重置计数: %v", err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "wrong-pass-1"); err != ErrInvalidCredentials {
		t.Fatalf("重置后再次错误应返回 ErrInvalidCredentials: %v", err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "password-1"); err != nil {
		t.Fatalf("计数未达阈值不应锁定: %v", err)
	}
}

func TestLoginBySMSAndReplay(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	registerAccount(t, service, "13812345678", "password-1")
	sms := NewSMSService()
	service.WithSMS(sms)
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "login", "654321", now); err != nil {
		t.Fatal(err)
	}
	if _, err := service.LoginBySMS(context.Background(), "13812345678", "654321", now); err != nil {
		t.Fatalf("验证码登录失败: %v", err)
	}
	if _, err := service.LoginBySMS(context.Background(), "13812345678", "654321", now); !errors.Is(err, ErrSMSInvalid) {
		t.Fatalf("验证码不可重放: %v", err)
	}
}

func TestResetPasswordBySMS(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	registerAccount(t, service, "13812345678", "password-1")
	sms := NewSMSService()
	service.WithSMS(sms)
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "reset", "112233", now); err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPasswordBySMS(context.Background(), "13812345678", "112233", "new-pass-1", now); err != nil {
		t.Fatalf("找回密码失败: %v", err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "new-pass-1"); err != nil {
		t.Fatalf("新密码应可登录: %v", err)
	}
}

func TestRebindPhone(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	registerAccount(t, service, "13812345678", "password-1")
	sms := NewSMSService()
	service.WithSMS(sms)
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "rebind_old", "111111", now); err != nil {
		t.Fatal(err)
	}
	if _, err := sms.Issue(context.Background(), "13912345678", "rebind_new", "222222", now); err != nil {
		t.Fatal(err)
	}
	if err := service.RebindPhone(context.Background(), 1, "111111", "13912345678", "222222", now); err != nil {
		t.Fatalf("换绑失败: %v", err)
	}
	if _, err := service.Login(context.Background(), "13912345678", "password-1"); err != nil {
		t.Fatalf("新手机号应可登录: %v", err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "password-1"); err != ErrInvalidCredentials {
		t.Fatalf("旧手机号应失效: %v", err)
	}
}

func TestRegisterConsumesSMS(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	sms := NewSMSService()
	service.WithSMS(sms)
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "register", "333333", now); err != nil {
		t.Fatal(err)
	}
	_, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != ErrInvalidInput {
		t.Fatalf("缺少验证码应返回 ErrInvalidInput: %v", err)
	}
	_, err = service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1", SMSCode: "333333", SMSScene: "register"})
	if err != nil {
		t.Fatalf("带验证码注册失败: %v", err)
	}
}

func TestAnonymizeAccount(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	registerAccount(t, service, "13812345678", "password-1")
	if err := service.AnonymizeAccount(context.Background(), 1); err != nil {
		t.Fatalf("注销失败: %v", err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "password-1"); err == nil {
		t.Fatalf("注销后不应可登录")
	}
}
