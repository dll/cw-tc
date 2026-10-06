package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSMSConsumeIsOneTime(t *testing.T) {
	service := NewSMSService()
	now := time.Unix(1000, 0)
	if _, err := service.Issue(context.Background(), "13812345678", "register", "123456", now); err != nil {
		t.Fatal(err)
	}
	if err := service.Consume("13812345678", "register", "123456", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(service.Consume("13812345678", "register", "123456", now.Add(2*time.Second)), ErrSMSInvalid) {
		t.Fatal("短信验证码消费后必须不可重放")
	}
}
