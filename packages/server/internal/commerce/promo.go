package commerce

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrPromoInvalid      = errors.New("优惠码无效或已过期")
	ErrPromoExhausted    = errors.New("优惠码已用完")
	ErrPromoStudentLimit = errors.New("该优惠码已达单人使用上限")
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
