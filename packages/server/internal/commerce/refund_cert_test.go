package commerce

import (
	"context"
	"testing"
)

// 铁律 4 / M3-08：normal 退款必须连带撤证，不得出现「退了款、证书仍有效」。
func TestRefundNormalRevokesCertificate(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)

	// 直接造一张有效证书（同 student_id=1, course_id=1）
	if _, err := database.Exec(`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at) VALUES('CW-CN-A202600001',1,1,'L1',80,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "normal", AmountCents: order.AmountCents, Reason: "重复购买", Initiator: "student"}); err != nil {
		t.Fatal(err)
	}

	var status, reason string
	if err := database.QueryRow(`SELECT status, revoked_reason FROM certificate WHERE cert_no='CW-CN-A202600001'`).Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "revoked" || reason != "order_refunded" {
		t.Fatalf("退款应撤证: status=%s reason=%s", status, reason)
	}
}

// reward 退款（免费学习）不得撤证：保留证书（铁律 8 · §7.3 G-7）。
func TestRefundRewardKeepsCertificate(t *testing.T) {
	database := commerceDB(t)
	defer database.Close()
	service := NewService(database)
	order := paidOrder(t, service)

	if _, err := database.Exec(`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at) VALUES('CW-CN-A202600002',1,1,'L1',80,'pass','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Refund(context.Background(), RefundInput{OrderNo: order.OrderNo, Kind: "reward", Basis: "campaign", BasisRef: "活动-1", AmountCents: order.AmountCents, Reason: "免费学习", Initiator: "admin", OperatorID: 1}); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := database.QueryRow(`SELECT status FROM certificate WHERE cert_no='CW-CN-A202600002'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "valid" {
		t.Fatalf("reward 退款应保留证书，实际 %s", status)
	}
}
