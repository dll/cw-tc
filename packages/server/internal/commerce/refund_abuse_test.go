package commerce

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 学员自助普通退款达上限后，第四次自助退款必须转人工审核（不自动退款）。
func TestRefundStudentAbuseRoutesToManualReview(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	now := time.Now().UTC()

	// 直接造 3 笔「学员自助 + normal」退款记录（跨 3 个已支付订单）。
	for i := 0; i < 3; i++ {
		orderNo := fmt.Sprintf("abuse-order-%d", i)
		if _, err := database.Exec(
			`INSERT INTO orders(order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at) VALUES(?,1,1,'样例课程',1000,1000,'paid',?)`,
			orderNo, now.Add(time.Hour).Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
		var orderID int64
		_ = database.QueryRow(`SELECT id FROM orders WHERE order_no=?`, orderNo).Scan(&orderID)
		if _, err := database.Exec(
			`INSERT INTO refund(refund_no, order_id, kind, amount_cents, reason, initiator, status, finished_at) VALUES(?,?,?,?,?,?,?,?)`,
			fmt.Sprintf("abuse-refund-%d", i), orderID, "normal", 1000, "事由", "student", "success", now.Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
	}

	// 第 4 个订单：学员自助退款 → 必须转人工审核。
	order := paidOrder(t, service)
	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "normal", AmountCents: order.AmountCents, Reason: "再次退款", Initiator: "student"}); err != ErrRefundNeedsManualReview {
		t.Fatalf("第 4 次自助退款应转人工审核，实际 err=%v", err)
	}
}
