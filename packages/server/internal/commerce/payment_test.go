package commerce

import (
	"context"
	"testing"
	"time"
)

func TestPaymentCallbackIsIdempotentAndGrantsEntitlement(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	order, err := NewService(database).CreateOrder(context.Background(), OrderInput{StudentID: 1, CourseID: 1, RequestID: "pay-1", CourseName: "样例课程", PriceCents: 1000, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	payment := NewPaymentService(database, []byte("callback-test-secret"))
	input := CallbackInput{OrderNo: order.OrderNo, Channel: "mock", ChannelTxnID: "txn-1", AmountCents: 1000, IdempotencyKey: "idem-1", Signature: SignCallback([]byte("callback-test-secret"), order.OrderNo, "txn-1", "1000")}
	if err := payment.HandleCallback(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := payment.HandleCallback(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	var payments, entitlements int
	if err := database.QueryRow(`SELECT COUNT(*) FROM payment_txn`).Scan(&payments); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM entitlement WHERE student_id=1 AND course_id=1 AND revoked_at IS NULL`).Scan(&entitlements); err != nil {
		t.Fatal(err)
	}
	if payments != 1 || entitlements != 1 {
		t.Fatalf("回调幂等结果错误: payment=%d entitlement=%d", payments, entitlements)
	}
}
