package identity

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// AnonymizeAccount 注销账号：撤销会话、置状态为 anonymized、匿名化姓名与证件字段。
// 手机号与身份证摘要替换为按学员 ID 派生的唯一占位值，保证唯一索引不冲突且不可逆。
func (s *Service) AnonymizeAccount(ctx context.Context, studentID int64) error {
	if s.db == nil || studentID <= 0 {
		return ErrInvalidCredentials
	}
	if err := s.RevokeAllSessions(ctx, studentID); err != nil {
		return err
	}
	synthetic := fmt.Sprintf("deleted-%d", studentID)
	digest := sha256.Sum256([]byte(synthetic))
	_, err := s.db.ExecContext(ctx,
		`UPDATE student SET status = 'anonymized', name = '已注销', phone = $1, id_card_cipher = X'', id_card_hash = $2, updated_at = $3 WHERE id = $4 AND status <> 'anonymized'`,
		synthetic, digest[:], time.Now().UTC().Format(time.RFC3339), studentID)
	return err
}
