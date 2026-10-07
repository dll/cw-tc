package commerce

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestActiveEntitlementsReturnsPaidCourseAccess(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	if _, err := database.Exec(`UPDATE course SET price_cents=1000 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "ent-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	payment := NewPaymentService(database, []byte("callback-test-secret"))
	callback := CallbackInput{OrderNo: order.OrderNo, Channel: "mock", ChannelTxnID: "txn-ent-1", AmountCents: order.AmountCents, IdempotencyKey: "idem-ent-1", Signature: SignCallback([]byte("callback-test-secret"), order.OrderNo, "txn-ent-1", strconv.FormatInt(order.AmountCents, 10))}
	if err := payment.HandleCallback(context.Background(), callback); err != nil {
		t.Fatal(err)
	}
	items, err := service.ListEntitlements(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].CourseID != 1 || items[0].Status != "active" {
		t.Fatalf("权益查询错误: %+v", items)
	}
}
