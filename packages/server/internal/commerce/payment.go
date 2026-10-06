package commerce

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrCallbackInvalid = errors.New("支付回调验签或金额校验失败")

type CallbackInput struct {
	OrderNo, Channel, ChannelTxnID, IdempotencyKey, Signature string
	AmountCents                                               int64
}
type PaymentService struct {
	db     *sql.DB
	secret []byte
}

func NewPaymentService(db *sql.DB, secret []byte) *PaymentService {
	return &PaymentService{db: db, secret: secret}
}
func SignCallback(secret []byte, orderNo, txnID, amount string) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(orderNo + "|" + txnID + "|" + amount))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *PaymentService) HandleCallback(ctx context.Context, in CallbackInput) error {
	if s.db == nil || in.OrderNo == "" || in.Channel == "" || in.ChannelTxnID == "" || in.IdempotencyKey == "" || in.AmountCents <= 0 || !hmac.Equal([]byte(in.Signature), []byte(SignCallback(s.secret, in.OrderNo, in.ChannelTxnID, strconv.FormatInt(in.AmountCents, 10)))) {
		return ErrCallbackInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orderID, studentID, courseID int64
	var status string
	var amount int64
	if err = tx.QueryRowContext(ctx, `SELECT id, student_id, course_id, amount_cents, status FROM orders WHERE order_no=?`, in.OrderNo).Scan(&orderID, &studentID, &courseID, &amount, &status); err != nil || amount != in.AmountCents {
		return ErrCallbackInvalid
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_txn WHERE idempotency_key=? OR (channel=? AND channel_txn_id=?)`, in.IdempotencyKey, in.Channel, in.ChannelTxnID).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO payment_txn(order_id, channel, channel_txn_id, amount_cents, raw_status, idempotency_key, callback_payload, verified_at) VALUES(?,?,?,?,?,?,?,?)`, orderID, in.Channel, in.ChannelTxnID, in.AmountCents, "success", in.IdempotencyKey, "[REDACTED]", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE orders SET status='paid', paid_at=?, updated_at=? WHERE id=? AND status='pending'`, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), orderID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO entitlement(student_id, course_id, source, order_id, granted_at) VALUES(?,?,?,?,?) ON CONFLICT(student_id, course_id) WHERE revoked_at IS NULL DO NOTHING`, studentID, courseID, "order", orderID, time.Now().UTC().Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "constraint") {
		return fmt.Errorf("开通权益失败: %w", err)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
