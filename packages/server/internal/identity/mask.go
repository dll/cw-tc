package identity

import (
	"context"
	"database/sql"
	"errors"

	"github.com/cw-tc/server/internal/redact"
)

// MaskedIDCard 返回学员身份证号的脱敏形式（前 6 + 8★ + 后 4）。
//
// 管理员详情页默认只展示脱敏值；看全号仍必须走 reveal（super + 事由 + 审计）。
// 未实名或密文不可解时返回空串，不阻断详情页其它数据。
func (s *Service) MaskedIDCard(ctx context.Context, studentID int64) (string, error) {
	if s == nil || s.db == nil || studentID <= 0 {
		return "", ErrInvalidCredentials
	}
	var ciphertext []byte
	if err := s.db.QueryRowContext(ctx,
		`SELECT id_card_cipher FROM student WHERE id = $1`, studentID).Scan(&ciphertext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if len(ciphertext) == 0 {
		return "", nil
	}
	plain, err := UnprotectIDCard(ciphertext, s.encKey)
	if err != nil {
		return "", nil
	}
	return redact.IDCard(plain), nil
}
