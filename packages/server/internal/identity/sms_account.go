package identity

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrSMSRequired = errors.New("短信服务未配置")

// LoginBySMS 使用手机号 + 短信验证码登录。
func (s *Service) LoginBySMS(ctx context.Context, phone, code string, now time.Time) (Student, error) {
	if s.sms == nil {
		return Student{}, ErrSMSRequired
	}
	if err := s.sms.Consume(phone, "login", code, now); err != nil {
		return Student{}, err
	}
	if s.db == nil {
		return Student{}, ErrDatabaseNotConfigured
	}
	var student Student
	err := s.db.QueryRowContext(ctx, `SELECT id, student_no, name, phone FROM student WHERE phone = $1 AND status = 'active'`, phone).
		Scan(&student.ID, &student.StudentNo, &student.Name, &student.Phone)
	if err != nil {
		return Student{}, ErrInvalidCredentials
	}
	return student, nil
}

// ResetPasswordBySMS 通过短信验证码找回密码。
func (s *Service) ResetPasswordBySMS(ctx context.Context, phone, code, newPassword string, now time.Time) error {
	if s.sms == nil {
		return ErrSMSRequired
	}
	if err := s.sms.Consume(phone, "reset", code, now); err != nil {
		return err
	}
	if s.db == nil {
		return ErrDatabaseNotConfigured
	}
	var id int64
	var idCardCipher []byte
	if err := s.db.QueryRowContext(ctx, `SELECT id, id_card_cipher FROM student WHERE phone = $1 AND status = 'active'`, phone).Scan(&id, &idCardCipher); err != nil {
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
	_, err = s.db.ExecContext(ctx, `UPDATE student SET password_hash = $1, password_updated_at = $2 WHERE id = $3`,
		hashPassword(newPassword), now.UTC().Format(time.RFC3339), id)
	return err
}

// RebindPhone 换绑手机号：旧号 + 新号双重短信验证。
func (s *Service) RebindPhone(ctx context.Context, studentID int64, oldCode, newPhone, newCode string, now time.Time) error {
	if s.sms == nil {
		return ErrSMSRequired
	}
	if ValidatePhone(newPhone) != nil {
		return ErrInvalidInput
	}
	if s.db == nil {
		return ErrDatabaseNotConfigured
	}
	var currentPhone string
	if err := s.db.QueryRowContext(ctx, `SELECT phone FROM student WHERE id = $1 AND status = 'active'`, studentID).Scan(&currentPhone); err != nil {
		return ErrInvalidCredentials
	}
	if err := s.sms.Consume(currentPhone, "rebind_old", oldCode, now); err != nil {
		return err
	}
	if err := s.sms.Consume(newPhone, "rebind_new", newCode, now); err != nil {
		return err
	}
	if s.studentExists(ctx, newPhone) {
		return ErrPhoneRegistered
	}
	_, err := s.db.ExecContext(ctx, `UPDATE student SET phone = $1, updated_at = $2 WHERE id = $3`, newPhone, now.UTC().Format(time.RFC3339), studentID)
	return err
}

var _ *sql.DB
