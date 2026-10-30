package commerce

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DefaultRefundSelfServiceDays 是「未参加测评」学员自助退款窗口（PRD §7.3 G-4）。
// 上线前迁入 system_config（pay.refund_self_service_days），不在业务层写死。
const DefaultRefundSelfServiceDays = 7

var (
	ErrRefundWindowExpired     = errors.New("已超过自助退款窗口，请联系客服")
	ErrRefundCertificateIssued = errors.New("已发证，退款需联系客服")
)

type StudentOrderRefund struct {
	RefundNo    string  `json:"refund_no"`
	AmountCents int64   `json:"amount_cents"`
	Status      string  `json:"status"`
	Reason      string  `json:"reason"`
	FailReason  *string `json:"fail_reason"`
	CreatedAt   string  `json:"created_at"`
	FinishedAt  *string `json:"finished_at"`
}

type StudentOrder struct {
	OrderNo        string              `json:"order_no"`
	CourseCode     string              `json:"course_code"`
	CourseName     string              `json:"course_name"`
	AmountCents    int64               `json:"amount_cents"`
	Status         string              `json:"status"`
	Channel        string              `json:"channel"`
	CreatedAt      string              `json:"created_at"`
	PaidAt         *string             `json:"paid_at"`
	ExpiredAt      string              `json:"expired_at"`
	RefundedAt     *string             `json:"refunded_at"`
	CanContinuePay bool                `json:"can_continue_pay"`
	CanCancel      bool                `json:"can_cancel"`
	CanRefund      bool                `json:"can_refund"`
	CanEnterCourse bool                `json:"can_enter_course"`
	ExamTaken      bool                `json:"exam_taken"`
	CertIssued     bool                `json:"cert_issued"`
	CertRevoked    bool                `json:"cert_revoked"`
	RefundableTill *string             `json:"refundable_until"`
	Refund         *StudentOrderRefund `json:"refund"`
}

const studentOrderQuery = `SELECT o.order_no, c.course_code, o.course_name_snapshot, o.amount_cents, o.status,
       COALESCE(o.channel,''), o.created_at, o.paid_at, o.expired_at, o.refunded_at,
       EXISTS(SELECT 1 FROM entitlement e WHERE e.student_id=o.student_id AND e.course_id=o.course_id AND e.revoked_at IS NULL),
       EXISTS(SELECT 1 FROM exam_result er WHERE er.student_id=o.student_id AND er.course_id=o.course_id),
       EXISTS(SELECT 1 FROM certificate ce WHERE ce.student_id=o.student_id AND ce.course_id=o.course_id AND ce.status='valid' AND ce.revoked_at IS NULL),
       EXISTS(SELECT 1 FROM certificate ce WHERE ce.student_id=o.student_id AND ce.course_id=o.course_id AND ce.revoked_reason='order_refunded'),
       r.refund_no, r.amount_cents, r.status, r.reason, r.fail_reason, r.created_at, r.finished_at
FROM orders o
JOIN course c ON c.id=o.course_id
LEFT JOIN refund r ON r.id=(SELECT r2.id FROM refund r2 WHERE r2.order_id=o.id ORDER BY r2.created_at DESC, r2.id DESC LIMIT 1)
WHERE o.student_id=$1`

// ListStudentOrders 返回学员订单列表（按最近下单排序），含可操作标记与退款进度。
func (s *Service) ListStudentOrders(ctx context.Context, studentID int64) ([]StudentOrder, error) {
	return s.queryStudentOrders(ctx, studentID, "")
}

// GetStudentOrder 返回单个订单；不存在或不属于该学员时返回 ErrOrderNotFound。
func (s *Service) GetStudentOrder(ctx context.Context, studentID int64, orderNo string) (StudentOrder, error) {
	items, err := s.queryStudentOrders(ctx, studentID, orderNo)
	if err != nil {
		return StudentOrder{}, err
	}
	if len(items) == 0 {
		return StudentOrder{}, ErrOrderNotFound
	}
	return items[0], nil
}

func (s *Service) queryStudentOrders(ctx context.Context, studentID int64, orderNo string) ([]StudentOrder, error) {
	if s.db == nil || studentID <= 0 {
		return nil, ErrOrderNotFound
	}
	query := studentOrderQuery + ` AND o.order_no=$2 ORDER BY o.created_at DESC, o.id DESC`
	args := []any{studentID, orderNo}
	if orderNo == "" {
		query = studentOrderQuery + ` ORDER BY o.created_at DESC, o.id DESC`
		args = []any{studentID}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	items := make([]StudentOrder, 0)
	for rows.Next() {
		var item StudentOrder
		var paidAt, refundedAt, refundNo, refundStatus, refundReason, refundFail, refundCreated, refundFinished sql.NullString
		var refundAmount sql.NullInt64
		var entitled, examTaken, certValid, certRevoked bool
		if err := rows.Scan(&item.OrderNo, &item.CourseCode, &item.CourseName, &item.AmountCents, &item.Status,
			&item.Channel, &item.CreatedAt, &paidAt, &item.ExpiredAt, &refundedAt,
			&entitled, &examTaken, &certValid, &certRevoked,
			&refundNo, &refundAmount, &refundStatus, &refundReason, &refundFail, &refundCreated, &refundFinished); err != nil {
			return nil, err
		}
		item.PaidAt = nullableText(paidAt)
		item.RefundedAt = nullableText(refundedAt)
		item.ExamTaken = examTaken
		item.CertIssued = certValid
		item.CertRevoked = certRevoked
		item.CanContinuePay = item.Status == "pending" && !isExpired(item.ExpiredAt, now)
		item.CanCancel = item.Status == "pending"
		item.CanEnterCourse = item.Status == "paid" && entitled

		var refundableTill time.Time
		if item.Status == "paid" && paidAt.Valid {
			paid, err := parseTime(paidAt.String)
			if err == nil {
				refundableTill = paid.Add(DefaultRefundSelfServiceDays * 24 * time.Hour)
				item.CanRefund = !certValid && (examTaken || now.Before(refundableTill))
				if !examTaken && !certValid {
					until := refundableTill.UTC().Format(time.RFC3339)
					item.RefundableTill = &until
				}
			}
		}

		if refundNo.Valid {
			item.Refund = &StudentOrderRefund{
				RefundNo:    refundNo.String,
				AmountCents: refundAmount.Int64,
				Status:      refundStatus.String,
				Reason:      refundReason.String,
				FailReason:  nullableText(refundFail),
				CreatedAt:   refundCreated.String,
				FinishedAt:  nullableText(refundFinished),
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// StudentRefund 学员自助退款（仅 normal；窗口与发证状态在此处兜底）。
func (s *Service) StudentRefund(ctx context.Context, studentID int64, orderNo, reason string) (Refund, error) {
	if s.db == nil || studentID <= 0 || orderNo == "" {
		return Refund{}, ErrOrderNotFound
	}
	var orderStudentID, orderCourseID, amountCents int64
	var status string
	var paidAt sql.NullString
	var examTaken, certValid bool
	err := s.db.QueryRowContext(ctx, `SELECT o.student_id,o.course_id,o.amount_cents,o.status,o.paid_at,
       EXISTS(SELECT 1 FROM exam_result er WHERE er.student_id=o.student_id AND er.course_id=o.course_id),
       EXISTS(SELECT 1 FROM certificate ce WHERE ce.student_id=o.student_id AND ce.course_id=o.course_id AND ce.status='valid' AND ce.revoked_at IS NULL)
       FROM orders o WHERE o.order_no=$1`, orderNo).Scan(&orderStudentID, &orderCourseID, &amountCents, &status, &paidAt, &examTaken, &certValid)
	if err == sql.ErrNoRows {
		return Refund{}, ErrOrderNotFound
	}
	if err != nil {
		return Refund{}, err
	}
	if orderStudentID != studentID {
		return Refund{}, ErrOrderNotFound
	}
	if status != "paid" {
		return Refund{}, ErrOrderNotRefundable
	}
	if certValid {
		return Refund{}, ErrRefundCertificateIssued
	}
	if !examTaken {
		if !paidAt.Valid {
			return Refund{}, ErrRefundWindowExpired
		}
		paid, err := parseTime(paidAt.String)
		if err != nil {
			return Refund{}, fmt.Errorf("订单支付时间异常: %w", err)
		}
		if time.Now().UTC().After(paid.Add(DefaultRefundSelfServiceDays * 24 * time.Hour)) {
			return Refund{}, ErrRefundWindowExpired
		}
	}
	if reason == "" {
		reason = "学员自助退款"
	}
	return s.Refund(ctx, RefundInput{
		OrderNo: orderNo, Kind: "normal", AmountCents: amountCents, Reason: reason,
		Initiator: "student", RequireNoCertificate: true,
	})
}

// LatestRefund 返回订单最近一次退款记录（不存在返回 nil）。
func (s *Service) LatestRefund(ctx context.Context, studentID int64, orderNo string) (*StudentOrderRefund, error) {
	if s.db == nil || studentID <= 0 || orderNo == "" {
		return nil, ErrOrderNotFound
	}
	var refund StudentOrderRefund
	var fail, finished sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT r.refund_no,r.amount_cents,r.status,r.reason,r.fail_reason,r.created_at,r.finished_at
       FROM refund r JOIN orders o ON o.id=r.order_id
       WHERE o.order_no=$1 AND o.student_id=$2
       ORDER BY r.created_at DESC, r.id DESC LIMIT 1`, orderNo, studentID).
		Scan(&refund.RefundNo, &refund.AmountCents, &refund.Status, &refund.Reason, &fail, &refund.CreatedAt, &finished)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	refund.FailReason = nullableText(fail)
	refund.FinishedAt = nullableText(finished)
	return &refund, nil
}

func nullableText(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func parseTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

func isExpired(value string, now time.Time) bool {
	expired, err := parseTime(value)
	return err != nil || now.After(expired)
}
