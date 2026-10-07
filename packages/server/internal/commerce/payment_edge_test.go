package commerce

import (
	"context"
	"testing"
)

func TestPaymentCallbackRejectsInvalidSignatureBeforeDatabaseWork(t *testing.T) {
	service := NewPaymentService(nil, []byte("callback-test-secret"))
	err := service.HandleCallback(context.Background(), CallbackInput{OrderNo: "O1", Channel: "mock", ChannelTxnID: "txn", IdempotencyKey: "idem", AmountCents: 1000, Signature: "bad"})
	if err != ErrCallbackInvalid {
		t.Fatalf("错误签名应拒绝: %v", err)
	}
}

func TestPaymentCallbackRejectsMissingRequiredFields(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewPaymentService(database, []byte("callback-test-secret"))
	if err := service.HandleCallback(context.Background(), CallbackInput{OrderNo: "O1"}); err != ErrCallbackInvalid {
		t.Fatalf("缺少支付回调字段应拒绝: %v", err)
	}
}
