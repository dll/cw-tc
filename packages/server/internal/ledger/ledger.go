// Package ledger 实现管理后台台账（PRD §7.10 · M5-08）：订单 / 退款 / 证书三张只读台账。
// 手机号一律脱敏（铁律 3）；身份证号永不返回。
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cw-tc/server/internal/redact"
)

var ErrLedgerInput = errors.New("台账参数非法")

// OrderRow 订单台账行。
type OrderRow struct {
	OrderNo     string `json:"order_no"`
	Phone       string `json:"phone"` // 脱敏
	CourseName  string `json:"course_name"`
	AmountCents int64  `json:"amount_cents"`
	Status      string `json:"status"`
	Channel     string `json:"channel"`
	PaidAt      string `json:"paid_at"`
	CreatedAt   string `json:"created_at"`
}

// ListOrders 订单台账：手机号脱敏，按状态过滤，分页。
func ListOrders(ctx context.Context, db *sql.DB, status string, limit, offset int) ([]OrderRow, error) {
	if db == nil || limit <= 0 {
		return nil, ErrLedgerInput
	}
	q := `SELECT o.order_no, s.phone, o.course_name_snapshot, o.amount_cents, o.status, o.channel, o.paid_at, o.created_at
	      FROM orders o JOIN student s ON o.student_id = s.id`
	args := []any{}
	if status != "" {
		q += ` WHERE o.status = $1`
		args = append(args, status)
	}
	n := len(args) + 1
	q += fmt.Sprintf(` ORDER BY o.id DESC LIMIT $%d OFFSET $%d`, n, n+1)
	args = append(args, limit, offset)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OrderRow, 0)
	for rows.Next() {
		var r OrderRow
		var channel, paidAt sql.NullString
		if err := rows.Scan(&r.OrderNo, &r.Phone, &r.CourseName, &r.AmountCents, &r.Status, &channel, &paidAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Phone = redact.Phone(r.Phone)
		if channel.Valid {
			r.Channel = channel.String
		}
		if paidAt.Valid {
			r.PaidAt = paidAt.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RefundRow 退款台账行。
type RefundRow struct {
	RefundNo    string `json:"refund_no"`
	OrderNo     string `json:"order_no"`
	Phone       string `json:"phone"` // 脱敏
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// ListRefunds 退款台账：手机号脱敏，分页。
func ListRefunds(ctx context.Context, db *sql.DB, limit, offset int) ([]RefundRow, error) {
	if db == nil || limit <= 0 {
		return nil, ErrLedgerInput
	}
	rows, err := db.QueryContext(ctx,
		`SELECT r.refund_no, o.order_no, s.phone, r.kind, r.amount_cents, r.reason, r.status, r.created_at
		 FROM refund r JOIN orders o ON r.order_id = o.id JOIN student s ON o.student_id = s.id
		 ORDER BY r.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RefundRow, 0)
	for rows.Next() {
		var r RefundRow
		if err := rows.Scan(&r.RefundNo, &r.OrderNo, &r.Phone, &r.Kind, &r.AmountCents, &r.Reason, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Phone = redact.Phone(r.Phone)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CertRow 证书台账行。
type CertRow struct {
	CertNo    string  `json:"cert_no"`
	Phone     string  `json:"phone"` // 脱敏
	Course    string  `json:"course"`
	Level     string  `json:"level"`
	Score     float64 `json:"score"`
	Band      string  `json:"band"`
	Status    string  `json:"status"`
	IssuedAt  string  `json:"issued_at"`
	ExpiresAt string  `json:"expires_at"`
}

// ListCertificates 证书台账：手机号脱敏，分页。
func ListCertificates(ctx context.Context, db *sql.DB, limit, offset int) ([]CertRow, error) {
	if db == nil || limit <= 0 {
		return nil, ErrLedgerInput
	}
	rows, err := db.QueryContext(ctx,
		`SELECT c.cert_no, s.phone, co.course_code, c.level, c.score, c.band, c.status, c.issued_at, c.expires_at
		 FROM certificate c JOIN student s ON c.student_id = s.id JOIN course co ON c.course_id = co.id
		 ORDER BY c.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CertRow, 0)
	for rows.Next() {
		var r CertRow
		if err := rows.Scan(&r.CertNo, &r.Phone, &r.Course, &r.Level, &r.Score, &r.Band, &r.Status, &r.IssuedAt, &r.ExpiresAt); err != nil {
			return nil, err
		}
		r.Phone = redact.Phone(r.Phone)
		out = append(out, r)
	}
	return out, rows.Err()
}
