// Package risk 实现风控初版（PRD §9.13）：黑名单 + 退款滥用限额。
//
// 原则（铁律「不封号优先」）：
//   - 黑名单粒度限 手机号 / 设备 / 支付账户，必须有期限、可申诉，永久黑名单不允许；
//   - 退款滥用超限「转人工审核」，不是禁止；
//   - 多账号套利只做关联提示，不硬拦（本包不实现硬性拒绝）。
package risk

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// TargetType 黑名单粒度（§9.13）。
type TargetType string

const (
	TargetPhone          TargetType = "phone"
	TargetDevice         TargetType = "device"
	TargetPaymentAccount TargetType = "payment_account"
)

// Status 黑名单状态。
type Status string

const (
	StatusActive   Status = "active"
	StatusExpired  Status = "expired"
	StatusAppealed Status = "appealed"
)

var (
	ErrBlacklistTarget = errors.New("黑名单粒度非法")
	ErrBlacklistInput  = errors.New("黑名单参数非法")
	ErrBlacklistEmpty  = errors.New("黑名单必须填违规原因与到期时间")
)

// AddBlacklist 新增黑名单条目：先解除同粒度同值的既有生效条目，再写入新条目（部分唯一索引兜底）。
func AddBlacklist(ctx context.Context, db *sql.DB, targetType TargetType, targetValue, reason string, expiresAt time.Time, createdBy int64) error {
	if db == nil {
		return errors.New("nil db")
	}
	if targetType != TargetPhone && targetType != TargetDevice && targetType != TargetPaymentAccount {
		return ErrBlacklistTarget
	}
	if targetValue == "" {
		return ErrBlacklistInput
	}
	if reason == "" || expiresAt.IsZero() {
		return ErrBlacklistEmpty
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 同粒度同值已有生效条目 → 先标记过期，再写新条目。
	if _, err := tx.ExecContext(ctx,
		`UPDATE risk_blacklist SET status = 'expired' WHERE target_type = ? AND target_value = ? AND status = 'active'`,
		string(targetType), targetValue); err != nil {
		return err
	}

	var createdByArg any
	if createdBy > 0 {
		createdByArg = createdBy
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO risk_blacklist(target_type, target_value, reason, status, expires_at, created_by) VALUES(?,?,?,?,?,?)`,
		string(targetType), targetValue, reason, string(StatusActive), expiresAt.UTC().Format(time.RFC3339), createdByArg)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// IsBlacklisted 判断目标当前是否在黑名单内：状态 active 且未到期。
func IsBlacklisted(ctx context.Context, db *sql.DB, targetType TargetType, targetValue string, now time.Time) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM risk_blacklist WHERE target_type = ? AND target_value = ? AND status = 'active' AND expires_at > ?`,
		string(targetType), targetValue, now.UTC().Format(time.RFC3339)).Scan(&n)
	return n > 0, err
}

// Appeal 对某条黑名单发起申诉：active → appealed，记录理由与时间。申诉期间暂不执行（IsBlacklisted 只认 active）。
func Appeal(ctx context.Context, db *sql.DB, id int64, appealReason string) error {
	if appealReason == "" {
		return ErrBlacklistInput
	}
	res, err := db.ExecContext(ctx,
		`UPDATE risk_blacklist SET status = 'appealed', appeal_reason = ?, appealed_at = ? WHERE id = ? AND status = 'active'`,
		appealReason, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("黑名单条目不存在或已解除")
	}
	return nil
}

// ResolveAppeal 管理员复核申诉：接受（→ expired 解除）或驳回（→ active 恢复）。
func ResolveAppeal(ctx context.Context, db *sql.DB, id int64, accept bool) error {
	next := StatusActive
	if accept {
		next = StatusExpired
	}
	res, err := db.ExecContext(ctx,
		`UPDATE risk_blacklist SET status = ? WHERE id = ? AND status = 'appealed'`, string(next), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("黑名单条目不存在或未处于申诉中")
	}
	return nil
}
