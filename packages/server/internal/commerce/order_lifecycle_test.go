package commerce

import (
	"context"
	"testing"
	"time"
)

func TestPreparePaymentRejectsAmountMismatchAndCancelClosesOrder(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "life-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.PreparePayment(context.Background(), 1, PaymentParams{OrderNo: order.OrderNo, Channel: "mock", AmountCents: order.AmountCents + 1}); err != ErrOrderNotPending {
		t.Fatalf("金额不一致应拒绝: %v", err)
	}
	if err = service.CancelOrder(context.Background(), 1, order.OrderNo); err != nil {
		t.Fatal(err)
	}
	closed, err := service.GetOrder(context.Background(), order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != "closed" {
		t.Fatalf("订单未关闭: %s", closed.Status)
	}
}
