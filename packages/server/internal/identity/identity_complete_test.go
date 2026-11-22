package identity

import (
	"context"
	"errors"
	"testing"
)

func TestCompleteIdentityRejectsMissingConfigAndBadInput(t *testing.T) {
	service := NewService(nil, nil, nil)
	if err := service.CompleteIdentity(context.Background(), 1, "110101199001011237"); !errors.Is(err, ErrDatabaseNotConfigured) {
		t.Fatalf("未配置数据库应拒绝，实际 %v", err)
	}
	service, database := accountTestService(t)
	defer database.Close()
	if err := service.CompleteIdentity(context.Background(), 0, "110101199001011237"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("非法学员 ID 应拒绝，实际 %v", err)
	}
	if err := service.CompleteIdentity(context.Background(), 1, "bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("非法身份证应拒绝，实际 %v", err)
	}
}

func TestCompleteIdentityOnlyOnceAndDeduplicates(t *testing.T) {
	service, database := accountTestService(t)
	defer database.Close()
	ctx := context.Background()

	student, err := service.Register(ctx, RegisterInput{
		Name: "样例学员", Phone: "13812345678", Password: "password-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if masked, err := service.MaskedIDCard(ctx, student.ID); err != nil || masked != "" {
		t.Fatalf("未实名时不应有脱敏号: %q err=%v", masked, err)
	}
	if err := service.CompleteIdentity(ctx, student.ID, "110101199001011237"); err != nil {
		t.Fatal(err)
	}
	if masked, err := service.MaskedIDCard(ctx, student.ID); err != nil || masked != "110101********1237" {
		t.Fatalf("脱敏结果 = %q err=%v", masked, err)
	}
	if err := service.CompleteIdentity(ctx, student.ID, "110101199003076755"); !errors.Is(err, ErrIdentityAlreadySet) {
		t.Fatalf("重复补充实名应拒绝，实际 %v", err)
	}
	if err := service.CompleteIdentity(ctx, student.ID, "bad-id"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("非法身份证应拒绝，实际 %v", err)
	}

	other, err := service.Register(ctx, RegisterInput{
		Name: "另一学员", Phone: "13812345679", Password: "password-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CompleteIdentity(ctx, other.ID, "110101199001011237"); !errors.Is(err, ErrIDCardRegistered) {
		t.Fatalf("同一身份证号应拒绝，实际 %v", err)
	}
}
