package commerce

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrOrderNotFound = errors.New("订单不存在")
var ErrOrderNotPending = errors.New("订单状态不允许此操作")

type PaymentParams struct {
	OrderNo     string
	Channel     string
	AmountCents int64
	ExpiresAt   time.Time
}

func (s *Service) GetOrder(ctx context.Context, orderNo string) (Order, error) {
	if s.db == nil || orderNo == "" {
		return Order{}, ErrOrderNotFound
	}
	var order Order
	var expired string
	err := s.db.QueryRowContext(ctx, `SELECT id, order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at FROM orders WHERE order_no=$1`, orderNo).Scan(&order.ID, &order.OrderNo, &order.StudentID, &order.CourseID, &order.CourseName, &order.PriceCents, &order.AmountCents, &order.Status, &expired)
	if err == sql.ErrNoRows {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}
	order.ExpiredAt, err = time.Parse(time.RFC3339, expired)
	return order, err
}

func (s *Service) PreparePayment(ctx context.Context, studentID int64, params PaymentParams) (Order, error) {
	order, err := s.GetOrder(ctx, params.OrderNo)
	if err != nil || order.StudentID != studentID {
		return Order{}, ErrOrderNotFound
	}
	if order.Status != "pending" || time.Now().UTC().After(order.ExpiredAt) || order.AmountCents != params.AmountCents {
		return Order{}, ErrOrderNotPending
	}
	if params.Channel != "wechat" && params.Channel != "alipay" && params.Channel != "mock" {
		return Order{}, ErrOrderNotPending
	}
	return order, nil
}

type Entitlement struct {
	ID         int64
	CourseID   int64
	CourseCode string
	CourseName string
	OrderID    int64
	Source     string
	Status     string
	GrantedAt  time.Time
}

func (s *Service) ListEntitlements(ctx context.Context, studentID int64) ([]Entitlement, error) {
	if s.db == nil || studentID <= 0 {
		return nil, ErrOrderNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.id, e.course_id, e.order_id, e.source, e.granted_at, c.course_code, c.name FROM entitlement e JOIN course c ON c.id = e.course_id WHERE e.student_id=$1 AND e.revoked_at IS NULL ORDER BY e.granted_at DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Entitlement, 0)
	for rows.Next() {
		var item Entitlement
		var granted string
		var orderID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.CourseID, &orderID, &item.Source, &granted, &item.CourseCode, &item.CourseName); err != nil {
			return nil, err
		}
		if orderID.Valid {
			item.OrderID = orderID.Int64
		}
		item.GrantedAt, err = time.Parse(time.RFC3339, granted)
		if err != nil {
			return nil, err
		}
		item.Status = "active"
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CancelOrder(ctx context.Context, studentID int64, orderNo string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE orders SET status='closed', closed_at=$1, updated_at=$2 WHERE order_no=$3 AND student_id=$4 AND status='pending'`, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), orderNo, studentID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrOrderNotPending
	}
	return nil
}

// MockPay 模拟支付成功（仅支付渠道线下未接时用，等价于支付回调的「置已支付 + 开通权益」）。
// 生产（已配置支付渠道）不应调用，接口层用 cfg.PaymentEnabled() 拦。
func (s *Service) MockPay(ctx context.Context, studentID int64, orderNo string) error {
	if s.db == nil || studentID <= 0 || orderNo == "" {
		return ErrOrderNotFound
	}
	order, err := s.GetOrder(ctx, orderNo)
	if err != nil || order.StudentID != studentID {
		return ErrOrderNotFound
	}
	if order.Status != "pending" {
		return ErrOrderNotPending
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = tx.ExecContext(ctx, `UPDATE orders SET status='paid', paid_at=$1, updated_at=$2 WHERE id=$3 AND status='pending'`, now, now, order.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO entitlement(student_id, course_id, source, order_id, granted_at) VALUES($1,$2,'order',$3,$4) ON CONFLICT(student_id, course_id) WHERE revoked_at IS NULL DO NOTHING`, studentID, order.CourseID, order.ID, now); err != nil {
		return err
	}
	return tx.Commit()
}
