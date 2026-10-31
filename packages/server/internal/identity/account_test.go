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

func TestLoginBySMSRequiresRegistrationReview(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	if _, err := service.Register(context.Background(), RegisterInput{
		Name: "样例学员", Phone: "13812345678", Password: "password-1", RequireReview: true,
	}); err != nil {
		t.Fatal(err)
	}
	sms := NewSMSService()
	service.WithSMS(sms)
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "login", "654321", now); err != nil {
		t.Fatal(err)
	}
	if _, err := service.LoginBySMS(context.Background(), "13812345678", "654321", now); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("待审核账号不得通过短信登录: %v", err)
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

func TestRegisterDoesNotConsumeSMS(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	sms := NewSMSService()
	service.WithSMS(sms)
	now := time.Now().UTC()
	if _, err := sms.Issue(context.Background(), "13812345678", "register", "333333", now); err != nil {
		t.Fatal(err)
	}
	_, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatalf("注册不应依赖短信验证码: %v", err)
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

func TestPasswordChangeAndSessionErrorBranches(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	ctx := context.Background()
	student := registerAccount(t, service, "13812345678", "password-1")

	if err := service.ChangePassword(ctx, 0, "password-1", "new-pass-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("非法学员改密应拒绝: %v", err)
	}
	if err := service.ChangePassword(ctx, student.ID, "password-1", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("空新密码应拒绝: %v", err)
	}
	if err := service.ChangePassword(ctx, student.ID, "wrong-old-1", "new-pass-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("旧密码错误应拒绝: %v", err)
	}
	// 新密码不得包含身份证片段（平台规则）。
	if err := service.ChangePassword(ctx, student.ID, "password-1", "199001011234aa"); err == nil {
		t.Fatal("弱密码应被拒绝")
	}
	if err := service.ChangePassword(ctx, student.ID, "password-1", "new-pass-1"); err != nil {
		t.Fatalf("正常改密失败: %v", err)
	}

	if _, err := service.CreateSessionWithDevice(ctx, 0, "ua", "ip", time.Now().UTC()); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("非法学员建会话应拒绝: %v", err)
	}
	if err := service.RevokeSession(ctx, ""); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("空 token 应拒绝: %v", err)
	}
	if err := service.RevokeSession(ctx, "missing-token"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("不存在会话应拒绝: %v", err)
	}
	session, err := service.CreateSession(ctx, student.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeSessionByID(ctx, student.ID+1, session.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("他人会话不可撤销: %v", err)
	}
	if err := service.RevokeSession(ctx, session.Token); err != nil {
		t.Fatalf("撤销会话失败: %v", err)
	}
	if err := service.RevokeSession(ctx, session.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("重复撤销应拒绝: %v", err)
	}
}

func TestProtectIDCardKeyAndPayloadErrors(t *testing.T) {
	if _, _, err := ProtectIDCard("110101199001011237", make([]byte, 16), make([]byte, 32)); err == nil {
		t.Fatal("短加密密钥应拒绝")
	}
	if _, err := UnprotectIDCard(make([]byte, 3), make([]byte, 32)); err == nil {
		t.Fatal("短密文应拒绝")
	}
	if _, err := UnprotectIDCard(make([]byte, 12), make([]byte, 16)); err == nil {
		t.Fatal("短解密密钥应拒绝")
	}
	// 合法加密后换错密钥解不开。
	ciphertext, digest, err := ProtectIDCard("110101199001011237", []byte("01234567890123456789012345678901"), []byte("hmac-key"))
	if err != nil || len(digest) != 32 {
		t.Fatalf("加密失败: %v", err)
	}
	if _, err := UnprotectIDCard(ciphertext, []byte("abcdefghijklmnopqrstuvwxyz123456")); err == nil {
		t.Fatal("错误密钥应解密失败")
	}
}
