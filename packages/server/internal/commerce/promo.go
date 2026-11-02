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
	ErrPromoInvalid      = errors.New("优惠码无效或已过期")
	ErrPromoExhausted    = errors.New("优惠码已用完")
	ErrPromoStudentLimit = errors.New("该优惠码已达单人使用上限")
	ErrPromoPendingOrder = errors.New("存在待支付订单，请取消后再使用优惠码")
)

type PromoInput struct {
	Code            string
	Kind            string
	Value           int64
	TotalLimit      int64 // -1 表示不限
	PerStudentLimit int64
	ValidFrom       time.Time
	ValidTo         time.Time
}

// CreatePromo 创建优惠码。
func (s *Service) CreatePromo(ctx context.Context, in PromoInput) error {
	if s.db == nil || in.Code == "" || (in.Kind != "amount_off" && in.Kind != "percent_off") || in.Value <= 0 {
		return ErrPromoInvalid
	}
	if in.PerStudentLimit <= 0 {
		in.PerStudentLimit = 1
	}
	var totalLimit any
	if in.TotalLimit >= 0 {
		totalLimit = in.TotalLimit
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO promo_code(code, kind, value, total_limit, per_student_limit, valid_from, valid_to, status) VALUES($1,$2,$3,$4,$5,$6,$7,'active')`,
		in.Code, in.Kind, in.Value, totalLimit, in.PerStudentLimit, in.ValidFrom.UTC().Format(time.RFC3339), in.ValidTo.UTC().Format(time.RFC3339))
	return err
}

// RedeemPromo 校验并核销优惠码，返回优惠金额；重复核销同一订单幂等返回同一结果。
func (s *Service) RedeemPromo(ctx context.Context, code string, studentID int64, orderNo string, priceCents int64, now time.Time) (int64, error) {
	if s.db == nil || code == "" || studentID <= 0 || orderNo == "" {
		return 0, ErrPromoInvalid
	}
	var kind string
	var value int64
	var totalLimit, perStudentLimit sql.NullInt64
	var validFrom, validTo string
	var status string
	if err := s.db.QueryRowContext(ctx,
		`SELECT kind, value, total_limit, per_student_limit, valid_from, valid_to, status FROM promo_code WHERE code = $1`, code).
		Scan(&kind, &value, &totalLimit, &perStudentLimit, &validFrom, &validTo, &status); err != nil {
		return 0, ErrPromoInvalid
	}
	if status != "active" {
		return 0, ErrPromoInvalid
	}
	from, err := time.Parse(time.RFC3339, validFrom)
	if err != nil {
		return 0, ErrPromoInvalid
	}
	to, err := time.Parse(time.RFC3339, validTo)
	if err != nil {
		return 0, ErrPromoInvalid
	}
	if now.Before(from) || now.After(to) {
		return 0, ErrPromoInvalid
	}
	// 已有核销记录 → 幂等返回同一折扣
	var existing sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT discount_cents FROM promo_redemption WHERE promo_code = $1 AND student_id = $2 AND order_no = $3`, code, studentID, orderNo).Scan(&existing); err == nil && existing.Valid {
		return existing.Int64, nil
	}
	// 总量限制
	if totalLimit.Valid {
		var used int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM promo_redemption WHERE promo_code = $1`, code).Scan(&used); err != nil {
			return 0, err
		}
		if int64(used) >= totalLimit.Int64 {
			return 0, ErrPromoExhausted
		}
	}
	// 单人限用
	limit := int64(1)
	if perStudentLimit.Valid {
		limit = perStudentLimit.Int64
	}
	var studentUsed int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM promo_redemption WHERE promo_code = $1 AND student_id = $2`, code, studentID).Scan(&studentUsed); err != nil {
		return 0, err
	}
	if int64(studentUsed) >= limit {
		return 0, ErrPromoStudentLimit
	}
	discount := promoDiscount(kind, value, priceCents)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO promo_redemption(promo_code, student_id, order_no, discount_cents, at) VALUES($1,$2,$3,$4,$5)`,
		code, studentID, orderNo, discount, now.UTC().Format(time.RFC3339)); err != nil {
		return 0, fmt.Errorf("记录优惠码核销失败: %w", err)
	}
	return discount, nil
}

// promoDiscount 计算优惠金额：amount_off 为固定分值；percent_off 的 value 为千分比（850 = 8.5 折）。
func promoDiscount(kind string, value, priceCents int64) int64 {
	switch kind {
	case "amount_off":
		if value > priceCents {
			return priceCents
		}
		return value
	case "percent_off":
		return priceCents * (1000 - value) / 1000
	default:
		return 0
	}
}

type promoState struct {
	kind            string
	value           int64
	totalLimit      sql.NullInt64
	perStudentLimit sql.NullInt64
	validFrom       time.Time
	validTo         time.Time
	status          string
}

func loadPromoState(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, code string) (promoState, error) {
	var state promoState
	var fromRaw, toRaw string
	if err := q.QueryRowContext(ctx,
		`SELECT kind, value, total_limit, per_student_limit, valid_from, valid_to, status FROM promo_code WHERE code = $1`, code).
		Scan(&state.kind, &state.value, &state.totalLimit, &state.perStudentLimit, &fromRaw, &toRaw, &state.status); err != nil {
		return state, ErrPromoInvalid
	}
	from, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return state, ErrPromoInvalid
	}
	to, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		return state, ErrPromoInvalid
	}
	state.validFrom, state.validTo = from, to
	return state, nil
}

func checkPromoUsage(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, state promoState, code string, studentID int64, now time.Time) error {
	if state.status != "active" || now.Before(state.validFrom) || now.After(state.validTo) {
		return ErrPromoInvalid
	}
	if state.totalLimit.Valid {
		var used int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM promo_redemption WHERE promo_code=$1`, code).Scan(&used); err != nil {
			return err
		}
		if int64(used) >= state.totalLimit.Int64 {
			return ErrPromoExhausted
		}
	}
	limit := int64(1)
	if state.perStudentLimit.Valid {
		limit = state.perStudentLimit.Int64
	}
	var usedByStudent int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM promo_redemption WHERE promo_code=$1 AND student_id=$2`, code, studentID).Scan(&usedByStudent); err != nil {
		return err
	}
	if int64(usedByStudent) >= limit {
		return ErrPromoStudentLimit
	}
	return nil
}

// ValidatePromo 只校验优惠码并返回可抵扣金额（不写库、不占额度）。
func (s *Service) ValidatePromo(ctx context.Context, code string, studentID, priceCents int64, now time.Time) (int64, error) {
	code = strings.TrimSpace(code)
	if s.db == nil || code == "" || studentID <= 0 || priceCents < 0 {
		return 0, ErrPromoInvalid
	}
	state, err := loadPromoState(ctx, s.db, code)
	if err != nil {
		return 0, err
	}
	if err := checkPromoUsage(ctx, s.db, state, code, studentID, now); err != nil {
		return 0, err
	}
	return promoDiscount(state.kind, state.value, priceCents), nil
}

// CreateOrderFromCourseWithPromo 在创建订单的同一事务内校验并核销优惠码。
// 已存在待支付订单时拒绝（避免订单金额与优惠码不一致）。
func (s *Service) CreateOrderFromCourseWithPromo(ctx context.Context, studentID, courseID int64, requestID, promoCode string, expiresAt time.Time) (Order, error) {
	promoCode = strings.TrimSpace(promoCode)
	if promoCode == "" {
		return s.CreateOrderFromCourse(ctx, studentID, courseID, requestID, expiresAt)
	}
	if s.db == nil || studentID <= 0 || courseID <= 0 || expiresAt.IsZero() {
		return Order{}, ErrOrderInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	var courseName string
	var price int64
	if err := tx.QueryRowContext(ctx, `SELECT name, price_cents FROM course WHERE id=$1 AND status='published'`, courseID).Scan(&courseName, &price); err != nil {
		return Order{}, fmt.Errorf("课程不存在或未上架: %w", err)
	}
	var pendingID int64
	var pendingExpired string
	if err := tx.QueryRowContext(ctx,
		`SELECT id, expired_at FROM orders WHERE student_id=$1 AND course_id=$2 AND status='pending'`,
		studentID, courseID).Scan(&pendingID, &pendingExpired); err == nil {
		expiredAtTime, _ := time.Parse(time.RFC3339, pendingExpired)
		if time.Now().Before(expiredAtTime) {
			return Order{}, ErrPromoPendingOrder
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET status='closed', closed_at=$1, updated_at=$1 WHERE id=$2 AND status='pending'`,
			time.Now().UTC().Format(time.RFC3339), pendingID); err != nil {
			return Order{}, err
		}
	}
	var studentDiscount int64
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(discount_cents,0) FROM student_course_discount WHERE student_id=$1 AND course_id=$2 AND active=1`, studentID, courseID).Scan(&studentDiscount)
	if studentDiscount > price {
		studentDiscount = price
	}
	now := time.Now().UTC()
	state, err := loadPromoState(ctx, tx, promoCode)
	if err != nil {
		return Order{}, err
	}
	if err := checkPromoUsage(ctx, tx, state, promoCode, studentID, now); err != nil {
		return Order{}, err
	}
	promoPart := promoDiscount(state.kind, state.value, price-studentDiscount)
	discount := studentDiscount + promoPart
	if discount > price {
		discount = price
	}
	orderNo := newOrderNo(studentID)
	var id int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO orders(order_no, student_id, course_id, course_name_snapshot, price_snapshot_cents, amount_cents, discount_cents, status, request_id, expired_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,'pending',$8,$9) RETURNING id`,
		orderNo, studentID, courseID, courseName, price, price-discount, discount, requestID,
		expiresAt.UTC().Format(time.RFC3339)).Scan(&id); err != nil {
		return Order{}, fmt.Errorf("创建订单失败: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO promo_redemption(promo_code, student_id, order_no, discount_cents, at) VALUES($1,$2,$3,$4,$5)`,
		promoCode, studentID, orderNo, promoPart, now.Format(time.RFC3339)); err != nil {
		return Order{}, fmt.Errorf("记录优惠码使用失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return Order{ID: id, OrderNo: orderNo, StudentID: studentID, CourseID: courseID, CourseName: courseName,
		PriceCents: price, DiscountCents: discount, AmountCents: price - discount, Status: "pending", ExpiredAt: expiresAt}, nil
}
