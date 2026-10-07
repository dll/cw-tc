package commerce

import (
	"context"
	"testing"
	"time"
)

func TestRefundErrorBranches(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	// nil db
	if _, err := NewService(nil).Refund(context.Background(), RefundInput{OrderNo: "O1", Kind: "normal", AmountCents: 100, Reason: "x", Initiator: "student"}); err != ErrRefundInput {
		t.Fatalf("nil db 应返回 ErrRefundInput: %v", err)
	}
	// 非法 kind
	order := paidOrder(t, service)
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "weird", AmountCents: order.AmountCents, Reason: "x", Initiator: "student"}); err != ErrRefundInput {
		t.Fatalf("非法 kind 应返回 ErrRefundInput: %v", err)
	}
	// 非法 initiator
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "normal", AmountCents: order.AmountCents, Reason: "x", Initiator: "hacker"}); err != ErrRefundInput {
		t.Fatalf("非法 initiator 应返回 ErrRefundInput: %v", err)
	}
	// admin 缺 operator
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "normal", AmountCents: order.AmountCents, Reason: "x", Initiator: "admin"}); err != ErrRefundInput {
		t.Fatalf("admin 缺 operator 应返回 ErrRefundInput: %v", err)
	}
	// 非 paid 订单不可退款
	pending, err := service.CreateOrder(context.Background(), OrderInput{StudentID: 1, CourseID: 1, RequestID: "pending-refund", CourseName: "样例课程", PriceCents: 1000, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: pending.OrderNo, Kind: "normal", AmountCents: 1000, Reason: "x", Initiator: "student"}); err != ErrOrderNotRefundable {
		t.Fatalf("非 paid 订单应返回 ErrOrderNotRefundable: %v", err)
	}
}
