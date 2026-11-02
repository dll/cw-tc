package commerce

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPromoValidateAndAtomicOrder(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := database.Exec(`UPDATE course SET price_cents=1000 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := service.CreatePromo(ctx, PromoInput{
		Code: "P300", Kind: "amount_off", Value: 300, TotalLimit: 2, PerStudentLimit: 1,
		ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	discount, err := service.ValidatePromo(ctx, "P300", 1, 1000, now)
	if err != nil || discount != 300 {
		t.Fatalf("校验优惠码失败: %d %v", discount, err)
	}
	order, err := service.CreateOrderFromCourseWithPromo(ctx, 1, 1, "req-promo", "P300", now.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if order.AmountCents != 700 || order.DiscountCents != 300 {
		t.Fatalf("优惠金额异常: %+v", order)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM promo_redemption WHERE promo_code='P300' AND order_no=$1`, order.OrderNo).Scan(&count); err != nil || count != 1 {
		t.Fatalf("优惠核销记录异常: %d %v", count, err)
	}
	if _, err := service.CreateOrderFromCourseWithPromo(ctx, 1, 1, "req2", "P300", now.Add(30*time.Minute)); !errors.Is(err, ErrPromoPendingOrder) {
		t.Fatalf("待支付订单应拒绝新优惠码订单: %v", err)
	}
	if _, err := service.ValidatePromo(ctx, "P300", 1, 1000, now); !errors.Is(err, ErrPromoStudentLimit) {
		t.Fatalf("单人限用应拒绝: %v", err)
	}
	if _, err := service.ValidatePromo(ctx, "NOPE", 1, 1000, now); !errors.Is(err, ErrPromoInvalid) {
		t.Fatalf("不存在优惠码应拒绝: %v", err)
	}
}

func TestPromoExpiredAndExhausted(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := service.CreatePromo(ctx, PromoInput{
		Code: "OLD", Kind: "percent_off", Value: 850, TotalLimit: 10, PerStudentLimit: 1,
		ValidFrom: now.Add(-2 * time.Hour), ValidTo: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidatePromo(ctx, "OLD", 1, 1000, now); !errors.Is(err, ErrPromoInvalid) {
		t.Fatalf("过期优惠码应拒绝: %v", err)
	}
	if err := service.CreatePromo(ctx, PromoInput{
		Code: "ONE", Kind: "amount_off", Value: 100, TotalLimit: 1, PerStudentLimit: 5,
		ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(2,'S00000002','样例学员二','13912345678',X'03',X'04','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateOrderFromCourseWithPromo(ctx, 1, 1, "r1", "ONE", now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidatePromo(ctx, "ONE", 2, 1000, now); !errors.Is(err, ErrPromoExhausted) {
		t.Fatalf("总量用尽应拒绝: %v", err)
	}
}
