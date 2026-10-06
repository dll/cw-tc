package commerce

import (
	"context"
	"testing"
	"time"
)

func TestOrderLifecycleAllowsPaymentPreparationAndRejectsUnknownChannel(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "life-2", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.PreparePayment(context.Background(), 1, PaymentParams{OrderNo: order.OrderNo, Channel: "mock", AmountCents: order.AmountCents})
	if err != nil || prepared.OrderNo != order.OrderNo {
		t.Fatalf("合法支付预下单失败: %+v %v", prepared, err)
	}
	if _, err = service.PreparePayment(context.Background(), 1, PaymentParams{OrderNo: order.OrderNo, Channel: "unknown", AmountCents: order.AmountCents}); err != ErrOrderNotPending {
		t.Fatalf("未知渠道应拒绝: %v", err)
	}
	if _, err = service.GetOrder(context.Background(), "missing"); err != ErrOrderNotFound {
		t.Fatalf("未知订单应返回 not found: %v", err)
	}
}
