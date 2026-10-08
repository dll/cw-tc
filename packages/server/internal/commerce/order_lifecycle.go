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
	ID        int64
	CourseID  int64
	OrderID   int64
	Source    string
	Status    string
	GrantedAt time.Time
}

func (s *Service) ListEntitlements(ctx context.Context, studentID int64) ([]Entitlement, error) {
	if s.db == nil || studentID <= 0 {
		return nil, ErrOrderNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, course_id, order_id, source, granted_at FROM entitlement WHERE student_id=$1 AND revoked_at IS NULL ORDER BY granted_at DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Entitlement, 0)
	for rows.Next() {
		var item Entitlement
		var granted string
		var orderID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.CourseID, &orderID, &item.Source, &granted); err != nil {
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
