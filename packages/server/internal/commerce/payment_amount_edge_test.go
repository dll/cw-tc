package commerce

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestPaymentCallbackRejectsAmountMismatch(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	order, err := NewService(database).CreateOrderFromCourse(context.Background(), 1, 1, "edge-amount", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	amount := order.AmountCents + 1
	service := NewPaymentService(database, []byte("callback-test-secret"))
	err = service.HandleCallback(context.Background(), CallbackInput{OrderNo: order.OrderNo, Channel: "mock", ChannelTxnID: "txn-edge", IdempotencyKey: "idem-edge", AmountCents: amount, Signature: SignCallback([]byte("callback-test-secret"), order.OrderNo, "txn-edge", strconv.FormatInt(amount, 10))})
	if err != ErrCallbackInvalid {
		t.Fatalf("金额不一致应拒绝: %v", err)
	}
}
