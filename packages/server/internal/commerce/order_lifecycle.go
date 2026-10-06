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
	err := s.db.QueryRowContext(ctx, `SELECT id, order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at FROM orders WHERE order_no=?`, orderNo).Scan(&order.ID, &order.OrderNo, &order.StudentID, &order.CourseID, &order.CourseName, &order.PriceCents, &order.AmountCents, &order.Status, &expired)
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

func (s *Service) CancelOrder(ctx context.Context, studentID int64, orderNo string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE orders SET status='closed', closed_at=?, updated_at=? WHERE order_no=? AND student_id=? AND status='pending'`, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), orderNo, studentID)
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
