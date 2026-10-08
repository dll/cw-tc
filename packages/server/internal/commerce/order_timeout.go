package commerce

import (
	"context"
	"time"
)

// CloseExpiredOrders 关闭已过期且仍为待支付的订单（幂等，只关 pending）。
func (s *Service) CloseExpiredOrders(ctx context.Context, now time.Time) (int64, error) {
	if s.db == nil {
		return 0, ErrOrderNotFound
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE orders SET status = 'closed', closed_at = $1, updated_at = $2 WHERE status = 'pending' AND expired_at < $3`,
		now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
