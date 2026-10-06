package commerce

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrOrderInput         = errors.New("订单参数非法")
	ErrPendingOrderExists = errors.New("已有待支付订单")
)

type OrderInput struct {
	StudentID  int64
	CourseID   int64
	RequestID  string
	CourseName string
	PriceCents int64
	ExpiresAt  time.Time
}

type Order struct {
	ID          int64
	OrderNo     string
	StudentID   int64
	CourseID    int64
	CourseName  string
	PriceCents  int64
	AmountCents int64
	Status      string
	ExpiredAt   time.Time
}

type Service struct{ db *sql.DB }

func NewService(database *sql.DB) *Service { return &Service{db: database} }

func scanPending(row *sql.Row, order *Order) error {
	var expired string
	if err := row.Scan(&order.ID, &order.OrderNo, &order.StudentID, &order.CourseID, &order.CourseName, &order.PriceCents, &order.AmountCents, &order.Status, &expired); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, expired)
	if err != nil {
		return err
	}
	order.ExpiredAt = parsed
	return nil
}

func (s *Service) pending(ctx context.Context, studentID, courseID int64) (Order, error) {
	var order Order
	err := scanPending(s.db.QueryRowContext(ctx, `SELECT id, order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at FROM orders WHERE student_id=? AND course_id=? AND status='pending'`, studentID, courseID), &order)
	return order, err
}

func (s *Service) CreateOrder(ctx context.Context, in OrderInput) (Order, error) {
	if s.db == nil || in.StudentID <= 0 || in.CourseID <= 0 || in.CourseName == "" || in.PriceCents < 0 || in.ExpiresAt.IsZero() {
		return Order{}, ErrOrderInput
	}
	existing, err := s.pending(ctx, in.StudentID, in.CourseID)
	if err == nil && time.Now().Before(existing.ExpiredAt) {
		return existing, nil
	}
	orderNo := fmt.Sprintf("O%d%08d", time.Now().UnixNano(), in.StudentID)
	result, err := s.db.ExecContext(ctx, `INSERT INTO orders(order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, request_id, expired_at) VALUES(?,?,?,?,?,?,?,?,?)`, orderNo, in.StudentID, in.CourseID, in.CourseName, in.PriceCents, in.PriceCents, "pending", in.RequestID, in.ExpiresAt.UTC().Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: orders.student_id, orders.course_id") {
		candidate, retryErr := s.pending(ctx, in.StudentID, in.CourseID)
		if retryErr == nil {
			return candidate, nil
		}
	}
	if err != nil {
		return Order{}, fmt.Errorf("创建订单失败: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Order{}, err
	}
	return Order{ID: id, OrderNo: orderNo, StudentID: in.StudentID, CourseID: in.CourseID, CourseName: in.CourseName, PriceCents: in.PriceCents, AmountCents: in.PriceCents, Status: "pending", ExpiredAt: in.ExpiresAt}, nil
}
