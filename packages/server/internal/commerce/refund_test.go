package commerce

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func paidOrder(t *testing.T, service *Service) Order {
	t.Helper()
	if _, err := service.db.Exec(`UPDATE course SET price_cents = 1000 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "refund-order", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// 直接通过回调把订单置为已支付并开通权益
	cb := CallbackInput{OrderNo: order.OrderNo, Channel: "mock", ChannelTxnID: "txn-" + order.OrderNo, AmountCents: order.AmountCents, IdempotencyKey: "idem-" + order.OrderNo}
	cb.Signature = SignCallback([]byte("callback-test-secret"), order.OrderNo, cb.ChannelTxnID, strconv.FormatInt(order.AmountCents, 10))
	svc := NewPaymentService(service.db, []byte("callback-test-secret"))
	if err := svc.HandleCallback(context.Background(), cb); err != nil {
		t.Fatal(err)
	}
	return order
}

func TestRefundNormalRevokesEntitlement(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)
	refund, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "normal", AmountCents: order.AmountCents, Reason: "重复购买", Initiator: "student"})
	if err != nil {
		t.Fatal(err)
	}
	if refund.Status != "success" {
		t.Fatalf("退款应成功: %+v", refund)
	}
	items, err := service.ListEntitlements(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("normal 退款应撤销权益: %+v", items)
	}
}

func TestRefundRewardKeepsEntitlement(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)
	refund, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "reward", Basis: "campaign", BasisRef: "活动-1", AmountCents: order.AmountCents, Reason: "奖励性退款", Initiator: "admin", OperatorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if refund.Status != "success" {
		t.Fatalf("退款应成功: %+v", refund)
	}
	items, err := service.ListEntitlements(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("reward 退款应保留权益: %+v", items)
	}
}

func TestRefundRejectsInvalidInput(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "reward", AmountCents: order.AmountCents, Reason: "缺依据", Initiator: "student"}); err != ErrRefundInput {
		t.Fatalf("reward 缺依据应拒绝: %v", err)
	}
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "normal", AmountCents: order.AmountCents + 1, Reason: "超额", Initiator: "student"}); err != ErrOrderNotRefundable {
		t.Fatalf("超额退款应拒绝: %v", err)
	}
}
