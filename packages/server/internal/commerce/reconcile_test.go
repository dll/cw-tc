package commerce

import (
	"context"
	"testing"
	"time"
)

func TestReconcileComputesDiffAndIsIdempotent(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	if _, err := database.Exec(`UPDATE course SET price_cents = 1000 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "reconcile-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// 直接把订单置为已支付
	if _, err := database.Exec(`UPDATE orders SET status='paid', paid_at=? WHERE order_no=?`, time.Now().UTC().Format(time.RFC3339), order.OrderNo); err != nil {
		t.Fatal(err)
	}
	bizDate := time.Now().UTC().Format("2006-01-02")
	result, err := service.Reconcile(context.Background(), bizDate, "mock", 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if result.DiffCount != 0 || result.DiffAmount != 0 {
		t.Fatalf("对账差异应为 0: %+v", result)
	}
	// 幂等：重复跑结果一致
	again, err := service.Reconcile(context.Background(), bizDate, "mock", 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if again.DiffCount != result.DiffCount || again.LocalCount != result.LocalCount {
		t.Fatalf("重复对账结果不一致: %+v vs %+v", result, again)
	}
}

func TestReconcileDetectsMismatch(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	bizDate := time.Now().UTC().Format("2006-01-02")
	result, err := service.Reconcile(context.Background(), bizDate, "mock", 3, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if result.DiffCount == 0 || result.DiffAmount == 0 {
		t.Fatalf("应检测到差异: %+v", result)
	}
}

func TestReconcileRejectsInvalidInput(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	if _, err := service.Reconcile(context.Background(), "", "mock", 1, 1000); err != ErrReconcileInput {
		t.Fatalf("空日期应返回 ErrReconcileInput: %v", err)
	}
	if _, err := service.Reconcile(context.Background(), "not-a-date", "mock", 1, 1000); err != ErrReconcileInput {
		t.Fatalf("非法日期应返回 ErrReconcileInput: %v", err)
	}
}
