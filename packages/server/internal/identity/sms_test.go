package identity

import (
	"context"
	"testing"
	"time"
)

func TestIssueSMSCodeHashAndCooldown(t *testing.T) {
	service := NewSMSService()
	first, err := service.Issue(context.Background(), "13812345678", "register", "123456", time.Unix(1000, 0))
	if err != nil || first == "123456" {
		t.Fatalf("验证码应哈希保存且首次成功: %q, %v", first, err)
	}
	if _, err := service.Issue(context.Background(), "13812345678", "register", "654321", time.Unix(1030, 0)); err == nil {
		t.Fatal("60 秒冷却内重复发送必须拒绝")
	}
	if !service.Verify("13812345678", "register", "123456", time.Unix(1061, 0)) {
		t.Fatal("冷却结束前生成的验证码应可验证")
	}
}
