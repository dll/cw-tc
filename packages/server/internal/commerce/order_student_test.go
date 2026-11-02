package commerce

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestListStudentOrdersAndLatestRefund(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)

	items, err := service.ListStudentOrders(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].OrderNo != order.OrderNo {
		t.Fatalf("订单列表异常: %+v", items)
	}
	item := items[0]
	if item.Status != "paid" || !item.CanEnterCourse || !item.CanRefund || item.CertIssued || item.ExamTaken {
		t.Fatalf("订单标记异常: %+v", item)
	}
	if item.RefundableTill == nil {
		t.Fatalf("未考试订单应返回退款窗口: %+v", item)
	}
	if refund, err := service.LatestRefund(context.Background(), 1, order.OrderNo); err != nil || refund != nil {
		t.Fatalf("退款前不应有记录: %+v %v", refund, err)
	}

	refund, err := service.StudentRefund(context.Background(), 1, order.OrderNo, "")
	if err != nil {
		t.Fatal(err)
	}
	if refund.Status != "success" {
		t.Fatalf("退款状态异常: %+v", refund)
	}
	latest, err := service.LatestRefund(context.Background(), 1, order.OrderNo)
	if err != nil || latest == nil || latest.RefundNo != refund.RefundNo {
		t.Fatalf("退款记录异常: %+v %v", latest, err)
	}
	items, err = service.ListStudentOrders(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Status != "refunded" || items[0].CanEnterCourse || items[0].CanRefund || items[0].Refund == nil {
		t.Fatalf("退款后订单标记异常: %+v", items[0])
	}
}

func TestStudentRefundRejectsCertificateAndExpiredWindow(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)

	if _, err := service.StudentRefund(context.Background(), 99, order.OrderNo, "x"); err != ErrOrderNotFound {
		t.Fatalf("他人订单应拒绝: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO certificate(cert_no,student_id,course_id,level,score,band,issued_at,expires_at) VALUES('CERT-X',1,1,'L1',90,'A','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StudentRefund(context.Background(), 1, order.OrderNo, "x"); err != ErrRefundCertificateIssued {
		t.Fatalf("已发证应拒绝自助退款: %v", err)
	}
	if _, err := database.Exec(`DELETE FROM certificate WHERE cert_no='CERT-X'`); err != nil {
		t.Fatal(err)
	}

	second, err := service.CreateOrderFromCourse(context.Background(), 1, 1, "expired", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.MockPay(context.Background(), 1, second.OrderNo); err != nil {
		t.Fatal(err)
	}
	oldPaid := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := database.Exec(`UPDATE orders SET paid_at=$1 WHERE id=$2`, oldPaid, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StudentRefund(context.Background(), 1, second.OrderNo, "x"); err != ErrRefundWindowExpired {
		t.Fatalf("超窗应拒绝自助退款: %v", err)
	}
}

func TestStudentOrderHelpers(t *testing.T) {
	if _, err := parseTime("2026-10-01T12:00:00.123Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseTime("2026-10-01T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if !isExpired("bad-time", time.Now()) {
		t.Fatal("非法时间应按已过期处理")
	}
	if nullableText(sql.NullString{}) != nil {
		t.Fatal("空值应返回 nil")
	}
	if got := nullableText(sql.NullString{String: "x", Valid: true}); got == nil || *got != "x" {
		t.Fatalf("有效值异常: %v", got)
	}
}

func TestGetStudentOrderOwnership(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	ctx := context.Background()
	order, err := service.CreateOrderFromCourse(ctx, 1, 1, "get-order", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetStudentOrder(ctx, 1, order.OrderNo)
	if err != nil || got.OrderNo != order.OrderNo {
		t.Fatalf("本人订单应可查: %+v %v", got, err)
	}
	if _, err := service.GetStudentOrder(ctx, 2, order.OrderNo); err != ErrOrderNotFound {
		t.Fatalf("他人订单应 404: %v", err)
	}
	if _, err := service.GetStudentOrder(ctx, 1, "NOPE"); err != ErrOrderNotFound {
		t.Fatalf("不存在订单应 404: %v", err)
	}
}
