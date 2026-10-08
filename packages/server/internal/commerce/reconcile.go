package commerce

import (
	"context"
	"errors"
	"time"
)

var ErrReconcileInput = errors.New("对账参数非法")

type ReconcileResult struct {
	BizDate       string
	Channel       string
	ChannelCount  int
	ChannelAmount int64
	LocalCount    int
	LocalAmount   int64
	DiffCount     int
	DiffAmount    int64
}

// Reconcile 按业务日期对账：本地已支付订单汇总 vs 渠道流水，幂等 upsert 到 reconcile_record。
// 差异为 0 是对账的硬要求；差异 ≠ 0 由调用方触发告警。
func (s *Service) Reconcile(ctx context.Context, bizDate, channel string, channelCount int, channelAmountCents int64) (ReconcileResult, error) {
	if s.db == nil || bizDate == "" || channel == "" {
		return ReconcileResult{}, ErrReconcileInput
	}
	start, err := time.Parse("2006-01-02", bizDate)
	if err != nil {
		return ReconcileResult{}, ErrReconcileInput
	}
	end := start.Add(24 * time.Hour)
	var localCount int
	var localAmount int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(amount_cents), 0) FROM orders WHERE status = 'paid' AND paid_at >= $1 AND paid_at < $2`,
		start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)).Scan(&localCount, &localAmount); err != nil {
		return ReconcileResult{}, err
	}
	diffCount := channelCount - localCount
	diffAmount := channelAmountCents - localAmount
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO reconcile_record(biz_date, channel, channel_count, channel_amount_cents, local_count, local_amount_cents, diff_count, diff_amount_cents) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT(biz_date, channel) DO UPDATE SET channel_count=excluded.channel_count, channel_amount_cents=excluded.channel_amount_cents, local_count=excluded.local_count, local_amount_cents=excluded.local_amount_cents, diff_count=excluded.diff_count, diff_amount_cents=excluded.diff_amount_cents`,
		bizDate, channel, channelCount, channelAmountCents, localCount, localAmount, diffCount, diffAmount)
	if err != nil {
		return ReconcileResult{}, err
	}
	return ReconcileResult{BizDate: bizDate, Channel: channel, ChannelCount: channelCount, ChannelAmount: channelAmountCents, LocalCount: localCount, LocalAmount: localAmount, DiffCount: diffCount, DiffAmount: diffAmount}, nil
}
