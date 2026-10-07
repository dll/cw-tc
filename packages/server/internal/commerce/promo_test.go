package commerce

import (
	"context"
	"testing"
	"time"
)

func TestPromoAmountOffAndIdempotent(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	now := time.Now().UTC()
	if err := service.CreatePromo(context.Background(), PromoInput{Code: "OFF100", Kind: "amount_off", Value: 100, TotalLimit: -1, PerStudentLimit: 1, ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	discount, err := service.RedeemPromo(context.Background(), "OFF100", 1, "order-1", 1000, now)
	if err != nil || discount != 100 {
		t.Fatalf("固定减额失败: %d %v", discount, err)
	}
	again, err := service.RedeemPromo(context.Background(), "OFF100", 1, "order-1", 1000, now)
	if err != nil || again != 100 {
		t.Fatalf("重复核销应幂等: %d %v", again, err)
	}
}

func TestPromoPercentOff(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	now := time.Now().UTC()
	if err := service.CreatePromo(context.Background(), PromoInput{Code: "P85", Kind: "percent_off", Value: 850, TotalLimit: -1, PerStudentLimit: 1, ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	discount, err := service.RedeemPromo(context.Background(), "P85", 1, "order-2", 1000, now)
	if err != nil || discount != 150 {
		t.Fatalf("折扣计算失败: %d %v", discount, err)
	}
}

func TestPromoLimitsAndExpiry(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	now := time.Now().UTC()
	if err := service.CreatePromo(context.Background(), PromoInput{Code: "LIMIT1", Kind: "amount_off", Value: 50, TotalLimit: 1, PerStudentLimit: 1, ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RedeemPromo(context.Background(), "LIMIT1", 1, "order-a", 1000, now); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RedeemPromo(context.Background(), "LIMIT1", 2, "order-b", 1000, now); err != ErrPromoExhausted {
		t.Fatalf("总量用尽应拒绝: %v", err)
	}
	if err := service.CreatePromo(context.Background(), PromoInput{Code: "EXPIRED", Kind: "amount_off", Value: 50, TotalLimit: -1, PerStudentLimit: 1, ValidFrom: now.Add(-2 * time.Hour), ValidTo: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RedeemPromo(context.Background(), "EXPIRED", 1, "order-c", 1000, now); err != ErrPromoInvalid {
		t.Fatalf("过期应拒绝: %v", err)
	}
	if _, err := service.RedeemPromo(context.Background(), "NOPE", 1, "order-d", 1000, now); err != ErrPromoInvalid {
		t.Fatalf("不存在应拒绝: %v", err)
	}
}
