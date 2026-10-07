package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"
)

var ErrInvalidCredentials = errors.New("手机号或密码错误")
var ErrAccountLocked = errors.New("账号已锁定，请稍后再试")

const (
	maxFailedLogins = 5
	lockDuration    = 10 * time.Minute
)

// Login 使用手机号 + 密码登录，内置连续失败锁定（按账号维度）。
func (s *Service) Login(ctx context.Context, phone, password string) (Student, error) {
	return s.loginWithClock(ctx, phone, password, time.Now().UTC())
}

func (s *Service) loginWithClock(ctx context.Context, phone, password string, now time.Time) (Student, error) {
	if s.db == nil || ValidatePhone(phone) != nil || password == "" {
		return Student{}, ErrInvalidCredentials
	}
	var student Student
	var stored []byte
	var lockedUntil sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, student_no, name, phone, password_hash, locked_until FROM student WHERE phone = ? AND status = 'active'`, phone).
		Scan(&student.ID, &student.StudentNo, &student.Name, &student.Phone, &stored, &lockedUntil)
	if err != nil {
		return Student{}, ErrInvalidCredentials
	}
	if lockedUntil.Valid && lockedUntil.String != "" {
		if t, perr := time.Parse(time.RFC3339, lockedUntil.String); perr == nil && now.Before(t) {
			return Student{}, ErrAccountLocked
		}
	}
	if len(stored) != 32 || subtle.ConstantTimeCompare(hashPassword(password), stored) != 1 {
		s.recordFailedLogin(ctx, student.ID, now)
		return Student{}, ErrInvalidCredentials
	}
	s.resetFailedLogin(ctx, student.ID, now)
	return student, nil
}

func (s *Service) recordFailedLogin(ctx context.Context, studentID int64, now time.Time) {
	_, _ = s.db.ExecContext(ctx, `UPDATE student SET failed_login_count = failed_login_count + 1, updated_at = ? WHERE id = ?`, now.UTC().Format(time.RFC3339), studentID)
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT failed_login_count FROM student WHERE id = ?`, studentID).Scan(&count); err == nil && count >= maxFailedLogins {
		_, _ = s.db.ExecContext(ctx, `UPDATE student SET locked_until = ? WHERE id = ?`, now.Add(lockDuration).UTC().Format(time.RFC3339), studentID)
	}
}

func (s *Service) resetFailedLogin(ctx context.Context, studentID int64, now time.Time) {
	_, _ = s.db.ExecContext(ctx, `UPDATE student SET failed_login_count = 0, locked_until = NULL, last_login_at = ? WHERE id = ?`, now.UTC().Format(time.RFC3339), studentID)
}

func (s *Service) studentExists(ctx context.Context, phone string) bool {
	if s.db == nil {
		return false
	}
	var id int64
	return s.db.QueryRowContext(ctx, `SELECT id FROM student WHERE phone = ?`, phone).Scan(&id) == nil
}

var _ *sql.DB
