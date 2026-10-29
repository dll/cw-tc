package identity

import (
	"context"
	"testing"
	"time"
)

func TestAdminResetCodeFlow(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	student := registerAccount(t, service, "13812345678", "password-1")
	ctx := context.Background()
	now := time.Now().UTC()

	session, err := service.CreateSession(ctx, student.ID, now)
	if err != nil {
		t.Fatal(err)
	}

	code, expires, last4, err := service.CreateAdminResetCode(ctx, student.ID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 8 || last4 != "5678" || !expires.After(now) {
		t.Fatalf("重置码返回异常: code=%q last4=%q expires=%v", code, last4, expires)
	}

	if err := service.ResetPasswordByAdminCode(ctx, "13812345678", "00000000", "new-pass-1", now); err != ErrResetCodeInvalid {
		t.Fatalf("错误重置码应拒绝: %v", err)
	}
	if err := service.ResetPasswordByAdminCode(ctx, "13812345678", code, "new-pass-1", now); err != nil {
		t.Fatalf("正确重置码应成功: %v", err)
	}
	if _, err := service.Login(ctx, "13812345678", "password-1"); err == nil {
		t.Fatal("旧密码应立即失效")
	}
	if _, err := service.Login(ctx, "13812345678", "new-pass-1"); err != nil {
		t.Fatalf("新密码应可登录: %v", err)
	}
	if _, err := service.Authenticate(ctx, session.Token, now); err == nil {
		t.Fatal("重置密码后旧会话应失效")
	}
	if err := service.ResetPasswordByAdminCode(ctx, "13812345678", code, "another-1", now); err != ErrResetCodeInvalid {
		t.Fatalf("重置码只能使用一次: %v", err)
	}
}

func TestResetPasswordByIdentity(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	student := registerAccount(t, service, "13812345678", "password-1")
	ctx := context.Background()
	now := time.Now().UTC()
	session, err := service.CreateSession(ctx, student.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := service.CreateAdminResetCode(ctx, student.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPasswordByIdentity(ctx, "13812345678", "110101199001011245", "new-pass-1", now); err != ErrResetCodeInvalid {
		t.Fatalf("身份证不匹配应拒绝: %v", err)
	}
	if err := service.ResetPasswordByIdentity(ctx, "13812345678", "110101199001011237", "new-pass-1", now); err != nil {
		t.Fatalf("身份证验证重置应成功: %v", err)
	}
	if _, err := service.Login(ctx, "13812345678", "password-1"); err == nil {
		t.Fatal("旧密码应立即失效")
	}
	if _, err := service.Login(ctx, "13812345678", "new-pass-1"); err != nil {
		t.Fatalf("新密码应可登录: %v", err)
	}
	if _, err := service.Authenticate(ctx, session.Token, now); err == nil {
		t.Fatal("重置后旧会话应失效")
	}
	if err := service.ResetPasswordByIdentity(ctx, "13812345678", "110101199001011237", "another-1", now); err != ErrResetCodeInvalid {
		t.Fatalf("授权只能使用一次: %v", err)
	}
}

func TestAdminResetCodeExpiredAndDisabled(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	student := registerAccount(t, service, "13812345678", "password-1")
	ctx := context.Background()
	now := time.Now().UTC()
	code, _, _, err := service.CreateAdminResetCode(ctx, student.ID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPasswordByAdminCode(ctx, "13812345678", code, "new-pass-1", now.Add(31*time.Minute)); err != ErrResetCodeInvalid {
		t.Fatalf("过期重置码应拒绝: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE student SET status='suspended' WHERE id=$1`, student.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := service.CreateAdminResetCode(ctx, student.ID, 1, now); err != ErrResetNotAllowed {
		t.Fatalf("停用账号不应允许重置: %v", err)
	}
}

func TestAdminResetCodeHelpers(t *testing.T) {
	if _, err := randomResetCode(5); err == nil {
		t.Fatal("位数过短应报错")
	}
	code, err := randomResetCode(8)
	if err != nil || len(code) != 8 {
		t.Fatalf("重置码生成失败: %q %v", code, err)
	}
	if emptyToNil("") != nil || emptyToNil("unknown") != nil || emptyToNil("ua") != "ua" {
		t.Fatal("emptyToNil 处理错误")
	}
	service, database := accountTestService(t)
	defer database.Close()
	if _, _, _, err := service.CreateAdminResetCode(context.Background(), 999, 1, time.Now().UTC()); err != ErrInvalidCredentials {
		t.Fatalf("不存在的学员应拒绝: %v", err)
	}
	if err := service.ResetPasswordByAdminCode(context.Background(), "bad", "12345678", "new-pass-1", time.Now().UTC()); err != ErrInvalidInput {
		t.Fatalf("非法手机号应拒绝: %v", err)
	}
}

func TestListAndRevokeSessions(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	student := registerAccount(t, service, "13812345678", "password-1")
	ctx := context.Background()
	now := time.Now().UTC()
	first, err := service.CreateSessionWithDevice(ctx, student.ID, "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0", "127.0.0.1", now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSessionWithDevice(ctx, student.ID, "Mozilla/5.0 (iPhone) Safari/605", "127.0.0.2", now)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.ListSessions(ctx, student.ID, first.Token, now)
	if err != nil || len(items) != 2 {
		t.Fatalf("设备列表错误: %d %v", len(items), err)
	}
	current := 0
	for _, item := range items {
		if item.Current {
			current++
		}
	}
	if current != 1 {
		t.Fatalf("当前会话标记错误: %d", current)
	}
	if err := service.RevokeSessionByID(ctx, student.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, second.Token, now); err == nil {
		t.Fatal("被踢设备会话应失效")
	}
	if err := service.RevokeSessionByID(ctx, student.ID+1, first.ID); err != ErrSessionNotFound {
		t.Fatal("不允许撤销他人会话")
	}
}
