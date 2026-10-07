package commerce

import (
	"context"
	"testing"
	"time"
)

func TestCloseExpiredOrdersClosesOnlyExpiredPending(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	now := time.Now().UTC()
	expired, err := service.CreateOrder(context.Background(), OrderInput{StudentID: 1, CourseID: 1, RequestID: "expired-1", CourseName: "样例课程", PriceCents: 1000, ExpiresAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	count, err := service.CloseExpiredOrders(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("应关闭 1 笔过期订单，实际 %d", count)
	}
	closed, err := service.GetOrder(context.Background(), expired.OrderNo)
	if err != nil || closed.Status != "closed" {
		t.Fatalf("过期订单应关闭: %+v %v", closed, err)
	}
}

func TestCreateOrderClosesExpiredPendingBeforeNew(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	now := time.Now().UTC()
	old, err := service.CreateOrder(context.Background(), OrderInput{StudentID: 1, CourseID: 1, RequestID: "old-1", CourseName: "样例课程", PriceCents: 1000, ExpiresAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "renew-1", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if renewed.OrderNo == old.OrderNo {
		t.Fatalf("过期订单应关闭并新建: %s", old.OrderNo)
	}
	oldAfter, err := service.GetOrder(context.Background(), old.OrderNo)
	if err != nil || oldAfter.Status != "closed" {
		t.Fatalf("旧订单应已关闭: %+v %v", oldAfter, err)
	}
}
