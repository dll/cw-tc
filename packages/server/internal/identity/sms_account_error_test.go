package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSMSAccountErrorBranches(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	now := time.Now().UTC()

	// 未注入短信服务时 fail-closed
	if _, err := service.LoginBySMS(context.Background(), "13812345678", "000000", now); !errors.Is(err, ErrSMSRequired) {
		t.Fatalf("未注入短信应返回 ErrSMSRequired: %v", err)
	}
	if err := service.ResetPasswordBySMS(context.Background(), "13812345678", "000000", "new-pass-1", now); !errors.Is(err, ErrSMSRequired) {
		t.Fatalf("未注入短信应返回 ErrSMSRequired: %v", err)
	}
	if err := service.RebindPhone(context.Background(), 1, "000000", "13912345678", "000000", now); !errors.Is(err, ErrSMSRequired) {
		t.Fatalf("未注入短信应返回 ErrSMSRequired: %v", err)
	}

	registerAccount(t, service, "13812345678", "password-1")
	sms := NewSMSService()
	service.WithSMS(sms)

	// 换绑新号非法
	if err := service.RebindPhone(context.Background(), 1, "111111", "bad", "222222", now); err != ErrInvalidInput {
		t.Fatalf("新号非法应返回 ErrInvalidInput: %v", err)
	}
	// 找回密码：不存在的号码
	if _, err := sms.Issue(context.Background(), "13812345678", "reset", "112233", now); err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPasswordBySMS(context.Background(), "13812345678", "112233", "weak", now); err == nil {
		t.Fatal("弱密码应被拒绝")
	}
}

func TestAnonymizeAccountErrorBranches(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	if err := service.AnonymizeAccount(context.Background(), 0); err != ErrInvalidCredentials {
		t.Fatalf("非法学员 ID 应返回 ErrInvalidCredentials: %v", err)
	}
	registerAccount(t, service, "13812345678", "password-1")
	if err := service.AnonymizeAccount(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := service.AnonymizeAccount(context.Background(), 1); err != nil {
		t.Fatalf("重复注销应幂等: %v", err)
	}
}
