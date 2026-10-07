package commerce

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/cw-tc/server/internal/risk"
)

var (
	ErrRefundInput             = errors.New("退款参数非法")
	ErrOrderNotRefundable      = errors.New("订单状态不允许退款")
	ErrRefundNeedsManualReview = errors.New("自助退款次数已达上限，转人工审核")
)

type RefundInput struct {
	OrderNo     string
	Kind        string
	Basis       string
	BasisRef    string
	AmountCents int64
	Reason      string
	Initiator   string
	OperatorID  int64
}

type Refund struct {
	RefundNo    string
	OrderNo     string
	Kind        string
	AmountCents int64
	Status      string
}

// Refund 创建退款：normal 撤销权益与证书；reward 保留权益与证书（奖励性退款）。
func (s *Service) Refund(ctx context.Context, in RefundInput) (Refund, error) {
	if s.db == nil || in.OrderNo == "" || in.AmountCents <= 0 || in.Reason == "" {
		return Refund{}, ErrRefundInput
	}
	if in.Kind != "normal" && in.Kind != "reward" {
		return Refund{}, ErrRefundInput
	}
	if in.Initiator != "student" && in.Initiator != "admin" {
		return Refund{}, ErrRefundInput
	}
	if in.Kind == "reward" && (in.Basis == "" || in.BasisRef == "") {
		return Refund{}, ErrRefundInput
	}
	if in.Initiator == "admin" && in.OperatorID <= 0 {
		return Refund{}, ErrRefundInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Refund{}, err
	}
	defer tx.Rollback()

	var orderID, studentID, courseID int64
	var amountCents int64
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT id, student_id, course_id, amount_cents, status FROM orders WHERE order_no = ?`, in.OrderNo).
		Scan(&orderID, &studentID, &courseID, &amountCents, &status); err != nil {
		return Refund{}, ErrOrderNotRefundable
	}
	if status != "paid" || in.AmountCents > amountCents {
		return Refund{}, ErrOrderNotRefundable
	}
	if in.Kind == "normal" && in.AmountCents != amountCents {
		return Refund{}, ErrOrderNotRefundable
	}

	// 退款滥用风控（§9.13）：学员自助普通退款达上限 → 转人工审核，不自动退款。
	if in.Initiator == "student" && in.Kind == "normal" {
		exceeded, err := risk.RefundAbuseExceeded(ctx, tx, studentID, risk.DefaultRefundAbuseLimit, risk.DefaultRefundAbuseWindowDays, time.Now())
		if err != nil {
			return Refund{}, err
		}
		if exceeded {
			return Refund{}, ErrRefundNeedsManualReview
		}
	}

	refundNo := newRefundNo()
	var operatorID any
	if in.OperatorID > 0 {
		operatorID = in.OperatorID
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO refund(refund_no, order_id, kind, basis, basis_ref, amount_cents, reason, initiator, operator_id, status, finished_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		refundNo, orderID, in.Kind, nullIfEmpty(in.Basis), nullIfEmpty(in.BasisRef), in.AmountCents, in.Reason, in.Initiator, operatorID, "success", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return Refund{}, fmt.Errorf("写入退款记录失败: %w", err)
	}

	if in.Kind == "normal" {
		if _, err := tx.ExecContext(ctx, `UPDATE entitlement SET revoked_at = ?, revoke_reason = 'order_refunded' WHERE student_id = ? AND course_id = ? AND revoked_at IS NULL`, time.Now().UTC().Format(time.RFC3339), studentID, courseID); err != nil {
			return Refund{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = 'refunded', refunded_at = ?, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), orderID); err != nil {
		return Refund{}, err
	}
	if err := tx.Commit(); err != nil {
		return Refund{}, err
	}
	return Refund{RefundNo: refundNo, OrderNo: in.OrderNo, Kind: in.Kind, AmountCents: in.AmountCents, Status: "success"}, nil
}

func newRefundNo() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return fmt.Sprintf("R%d%s", time.Now().UnixNano(), hex.EncodeToString(raw[:]))
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ = sql.ErrNoRows
