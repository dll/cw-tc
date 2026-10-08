package identity

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"
)

// ChangePassword 校验旧密码后更新为新密码，旧密码立即失效。
// 新密码经过与注册一致的强度校验，并拦截身份证后 6 位 / 生日弱口令。
func (s *Service) ChangePassword(ctx context.Context, studentID int64, oldPassword, newPassword string) error {
	if s.db == nil || studentID <= 0 || newPassword == "" {
		return ErrInvalidCredentials
	}
	var stored []byte
	var idCardCipher []byte
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash, id_card_cipher FROM student WHERE id = $1`, studentID).Scan(&stored, &idCardCipher); err != nil {
		return ErrInvalidCredentials
	}
	if len(stored) != 32 || subtle.ConstantTimeCompare(hashPassword(oldPassword), stored) != 1 {
		return ErrInvalidCredentials
	}
	idCard, err := UnprotectIDCard(idCardCipher, s.encKey)
	if err == nil {
		if err := ValidatePasswordAgainstIDCard(newPassword, idCard); err != nil {
			return err
		}
	} else if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	updated, err := s.db.ExecContext(ctx, `UPDATE student SET password_hash = $1, password_updated_at = $2 WHERE id = $3`,
		hashPassword(newPassword), time.Now().UTC().Format(time.RFC3339), studentID)
	if err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}
	if affected, err := updated.RowsAffected(); err == nil && affected == 0 {
		return ErrInvalidCredentials
	}
	return nil
}
