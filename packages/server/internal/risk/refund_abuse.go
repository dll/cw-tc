package risk

import (
	"context"
	"database/sql"
	"time"
)

// 退款滥用限额默认值（§9.13：首发建议 3 次/年；上线前迁入 system_config）。
const (
	DefaultRefundAbuseLimit      = 3
	DefaultRefundAbuseWindowDays = 365
)

// queryRower 抽象 QueryRowContext，兼容 *sql.DB 与 *sql.Tx（避免事务内再查 db 造成的单连接死锁）。
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// RefundAbuseExceeded 判断学员在统计窗口内的自助退款次数是否已达上限。
// 只统计「学员自助 + 普通退款」（initiator='student' 且 kind='normal'）；奖励性退款（reward）不计入滥用。
func RefundAbuseExceeded(ctx context.Context, q queryRower, studentID int64, limit int, windowDays int, now time.Time) (bool, error) {
	if q == nil {
		return false, nil
	}
	threshold := now.AddDate(0, 0, -windowDays).UTC().Format(time.RFC3339)
	var count int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM refund r JOIN orders o ON r.order_id = o.id
		 WHERE o.student_id = ? AND r.initiator = 'student' AND r.kind = 'normal' AND r.finished_at >= ?`,
		studentID, threshold).Scan(&count); err != nil {
		return false, err
	}
	return count >= limit, nil
}
