package commerce

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// newOrderNo 生成唯一订单号：时间戳 + 学员号 + 随机段，避免并发或粗粒度时钟下碰撞。
func newOrderNo(studentID int64) string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return fmt.Sprintf("O%d%08d%s", time.Now().UnixNano(), studentID, hex.EncodeToString(raw[:]))
}

var (
	ErrOrderInput         = errors.New("订单参数非法")
	ErrPendingOrderExists = errors.New("已有待支付订单")
)

type OrderInput struct {
	StudentID     int64
	CourseID      int64
	RequestID     string
	CourseName    string
	PriceCents    int64
	DiscountCents int64
	ExpiresAt     time.Time
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
	err := scanPending(s.db.QueryRowContext(ctx, `SELECT id, order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, status, expired_at FROM orders WHERE student_id=$1 AND course_id=$2 AND status='pending'`, studentID, courseID), &order)
	return order, err
}

func (s *Service) CreateOrderFromCourse(ctx context.Context, studentID, courseID int64, requestID string, expiresAt time.Time) (Order, error) {
	if s.db == nil || studentID <= 0 || courseID <= 0 {
		return Order{}, ErrOrderInput
	}
	var input OrderInput
	if err := s.db.QueryRowContext(ctx, `SELECT name, price_cents FROM course WHERE id=$1 AND status='published'`, courseID).Scan(&input.CourseName, &input.PriceCents); err != nil {
		return Order{}, fmt.Errorf("课程不可下单: %w", err)
	}
	input.StudentID, input.CourseID, input.RequestID, input.ExpiresAt = studentID, courseID, requestID, expiresAt
	var discount int64
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(discount_cents,0) FROM student_course_discount WHERE student_id=$1 AND course_id=$2 AND active=1`, studentID, courseID).Scan(&discount)
	if discount > input.PriceCents {
		discount = input.PriceCents
	}
	input.DiscountCents = discount
	return s.CreateOrder(ctx, input)
}

func (s *Service) CreateOrder(ctx context.Context, in OrderInput) (Order, error) {
	if s.db == nil || in.StudentID <= 0 || in.CourseID <= 0 || in.CourseName == "" || in.PriceCents < 0 || in.DiscountCents < 0 || in.DiscountCents > in.PriceCents || in.ExpiresAt.IsZero() {
		return Order{}, ErrOrderInput
	}
	existing, err := s.pending(ctx, in.StudentID, in.CourseID)
	if err == nil && time.Now().Before(existing.ExpiredAt) {
		return existing, nil
	}
	// 待支付订单已过期：先关闭，再创建新订单，避免唯一索引冲突。
	if err == nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE orders SET status='closed', closed_at=$1, updated_at=$2 WHERE id=$3 AND status='pending'`,
			time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), existing.ID)
	}
	orderNo := newOrderNo(in.StudentID)
	var id int64
	err = s.db.QueryRowContext(ctx, `INSERT INTO orders(order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, discount_cents, status, request_id, expired_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, orderNo, in.StudentID, in.CourseID, in.CourseName, in.PriceCents, in.PriceCents-in.DiscountCents, in.DiscountCents, "pending", in.RequestID, in.ExpiresAt.UTC().Format(time.RFC3339)).Scan(&id)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: orders.student_id, orders.course_id") {
		candidate, retryErr := s.pending(ctx, in.StudentID, in.CourseID)
		if retryErr == nil {
			return candidate, nil
		}
	}
	if err != nil {
		return Order{}, fmt.Errorf("创建订单失败: %w", err)
	}
	return Order{ID: id, OrderNo: orderNo, StudentID: in.StudentID, CourseID: in.CourseID, CourseName: in.CourseName, PriceCents: in.PriceCents, AmountCents: in.PriceCents - in.DiscountCents, Status: "pending", ExpiredAt: in.ExpiresAt}, nil
}
