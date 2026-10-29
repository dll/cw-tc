package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"
)

var (
	ErrResetCodeInvalid = errors.New("重置码无效或已过期")
	ErrResetNotAllowed  = errors.New("当前账号状态不允许重置密码")
)

const adminResetCodeTTL = 30 * time.Minute

// CreateAdminResetCode 由管理员生成一次性重置码（30 分钟）。
// 只返回重置码给管理员转交，管理员永远看不到新密码；库里只存哈希。
func (s *Service) CreateAdminResetCode(ctx context.Context, studentID, createdBy int64, now time.Time) (code string, expires time.Time, phoneLast4 string, err error) {
	if s.db == nil || studentID <= 0 || createdBy <= 0 {
		return "", time.Time{}, "", ErrInvalidCredentials
	}
	var phone, status string
	if err = s.db.QueryRowContext(ctx,
		`SELECT phone, status FROM student WHERE id = $1`, studentID).Scan(&phone, &status); err != nil {
		if err == sql.ErrNoRows {
			return "", time.Time{}, "", ErrInvalidCredentials
		}
		return "", time.Time{}, "", err
	}
	if status != "active" {
		return "", time.Time{}, "", ErrResetNotAllowed
	}
	code, err = randomResetCode(8)
	if err != nil {
		return "", time.Time{}, "", err
	}
	digest := sha256.Sum256([]byte("admin-reset:" + code))
	expires = now.Add(adminResetCodeTTL)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, "", err
	}
	defer tx.Rollback()
	// 同一手机号只保留最新一条：旧的未使用重置码立即作废。
	if _, err = tx.ExecContext(ctx,
		`UPDATE sms_code SET consumed_at = $1 WHERE phone = $2 AND scene = 'reset' AND consumed_at IS NULL`,
		now.UTC().Format(time.RFC3339), phone); err != nil {
		return "", time.Time{}, "", err
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO sms_code(phone, scene, code_hash, expires_at, created_at) VALUES($1,'reset',$2,$3,$4)`,
		phone, digest[:], expires.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339)); err != nil {
		return "", time.Time{}, "", err
	}
	// 站内送达：写入学员消息中心，避免管理员通过微信/电话等站外渠道转交。
	body := fmt.Sprintf("你的密码重置码：%s，有效期至 %s。请在登录页「忘记密码」中输入重置码设置新密码；请勿转发给他人。",
		code, expires.Format(time.RFC3339))
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO notification(student_id, type, title, body, link) VALUES($1,'system','密码重置码',$2,'/reset-password')`,
		studentID, body); err != nil {
		return "", time.Time{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return "", time.Time{}, "", err
	}
	if len(phone) >= 4 {
		phoneLast4 = phone[len(phone)-4:]
	}
	return code, expires, phoneLast4, nil
}

// ResetPasswordByAdminCode 校验管理员下发的重置码并设置新密码。
// 重置成功后旧会话全部失效，避免旧设备继续使用。
func (s *Service) ResetPasswordByAdminCode(ctx context.Context, phone, code, newPassword string, now time.Time) error {
	if s.db == nil {
		return ErrDatabaseNotConfigured
	}
	if ValidatePhone(phone) != nil || code == "" || newPassword == "" {
		return ErrInvalidInput
	}
	digest := sha256.Sum256([]byte("admin-reset:" + code))
	var codeID int64
	var expiresAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, expires_at FROM sms_code WHERE phone = $1 AND scene = 'reset' AND code_hash = $2 AND consumed_at IS NULL ORDER BY created_at DESC, id DESC LIMIT 1`,
		phone, digest[:]).Scan(&codeID, &expiresAt)
	if err == sql.ErrNoRows {
		return ErrResetCodeInvalid
	}
	if err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || !now.Before(expires) {
		return ErrResetCodeInvalid
	}
	var id int64
	var idCardCipher []byte
	if err := s.db.QueryRowContext(ctx,
		`SELECT id, id_card_cipher FROM student WHERE phone = $1 AND status = 'active'`, phone).Scan(&id, &idCardCipher); err != nil {
		return ErrInvalidCredentials
	}
	if idCard, unprotectErr := UnprotectIDCard(idCardCipher, s.encKey); unprotectErr == nil {
		if err := ValidatePasswordAgainstIDCard(newPassword, idCard); err != nil {
			return err
		}
	} else if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx,
		`UPDATE student SET password_hash = $1, password_updated_at = $2, failed_login_count = 0, locked_until = NULL WHERE id = $3`,
		hashPassword(newPassword), now.UTC().Format(time.RFC3339), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE sms_code SET consumed_at = $1 WHERE id = $2 AND consumed_at IS NULL`,
		now.UTC().Format(time.RFC3339), codeID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE student_session SET revoked_at = CURRENT_TIMESTAMP WHERE student_id = $1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetPasswordByIdentity 学员用手机号 + 身份证号验证身份后直接重置密码。
// 前提：管理员已在 30 分钟内为该手机号生成过重置授权；码本身无需站外转交。
func (s *Service) ResetPasswordByIdentity(ctx context.Context, phone, idCard, newPassword string, now time.Time) error {
	if s.db == nil {
		return ErrDatabaseNotConfigured
	}
	if ValidatePhone(phone) != nil || ValidateIDCard(idCard) != nil || newPassword == "" {
		return ErrInvalidInput
	}
	var studentID int64
	var stored []byte
	if err := s.db.QueryRowContext(ctx,
		`SELECT id, id_card_hash FROM student WHERE phone = $1 AND status = 'active'`, phone).Scan(&studentID, &stored); err != nil {
		return ErrResetCodeInvalid
	}
	if len(stored) == 0 {
		return ErrResetCodeInvalid
	}
	_, digest, err := ProtectIDCard(idCard, s.encKey, s.hmacKey)
	if err != nil || subtle.ConstantTimeCompare(digest, stored) != 1 {
		return ErrResetCodeInvalid
	}
	var codeID int64
	var expiresAt string
	err = s.db.QueryRowContext(ctx,
		`SELECT id, expires_at FROM sms_code WHERE phone = $1 AND scene = 'reset' AND consumed_at IS NULL ORDER BY created_at DESC, id DESC LIMIT 1`,
		phone).Scan(&codeID, &expiresAt)
	if err != nil {
		return ErrResetCodeInvalid
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || !now.Before(expires) {
		return ErrResetCodeInvalid
	}
	if err := ValidatePasswordAgainstIDCard(newPassword, idCard); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx,
		`UPDATE student SET password_hash = $1, password_updated_at = $2, failed_login_count = 0, locked_until = NULL WHERE id = $3`,
		hashPassword(newPassword), now.UTC().Format(time.RFC3339), studentID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE sms_code SET consumed_at = $1 WHERE id = $2 AND consumed_at IS NULL`,
		now.UTC().Format(time.RFC3339), codeID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE student_session SET revoked_at = CURRENT_TIMESTAMP WHERE student_id = $1 AND revoked_at IS NULL`, studentID); err != nil {
		return err
	}
	return tx.Commit()
}

func randomResetCode(digits int) (string, error) {
	if digits < 6 || digits > 12 {
		return "", fmt.Errorf("重置码位数非法")
	}
	max := int64(1)
	for i := 0; i < digits; i++ {
		max *= 10
	}
	value, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, value.Int64()), nil
}
