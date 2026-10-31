package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCredentials = errors.New("手机号或密码错误")
var ErrRegistrationPending = errors.New("账号待运营管理员审核，请审核通过后登录")
var ErrAccountLocked = errors.New("账号已锁定，请稍后再试")

// AccountDisabledError 表示账号被停用 / 封禁；登录页需展示原因类别与申诉入口。
type AccountDisabledError struct {
	Status     string
	ReasonKind string
	Until      *time.Time
}

func (e *AccountDisabledError) Error() string {
	if e.Status == "banned" {
		return "账号已被封禁，可发起申诉"
	}
	return "账号已停用，可发起申诉"
}

const (
	maxFailedLogins = 5
	lockDuration    = 10 * time.Minute
)

// Login 使用手机号 + 密码登录，内置连续失败锁定（按账号维度）。
func (s *Service) Login(ctx context.Context, phone, password string) (Student, error) {
	return s.loginWithClock(ctx, phone, password, time.Now().UTC())
}

func (s *Service) loginWithClock(ctx context.Context, phone, password string, now time.Time) (Student, error) {
	phone = strings.TrimSpace(phone)
	if s.db == nil || ValidatePhone(phone) != nil || password == "" {
		return Student{}, ErrInvalidCredentials
	}
	var student Student
	var stored string
	var lockedUntil sql.NullString
	var reviewStatus, status string
	var reasonKind, until sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, student_no, name, phone, password_hash, locked_until, registration_review_status,
		        status, status_reason_kind, status_until
		 FROM student WHERE phone = $1`, phone).
		Scan(&student.ID, &student.StudentNo, &student.Name, &student.Phone, &stored, &lockedUntil, &reviewStatus,
			&status, &reasonKind, &until)
	if err != nil {
		return Student{}, ErrInvalidCredentials
	}
	// 停用 / 封禁必须在登录时如实告知原因与申诉入口，不允许“莫名登不上”（PRD §7.10.10（三））。
	if status == "suspended" || status == "banned" || status == "limited" {
		disabled := &AccountDisabledError{Status: status, ReasonKind: reasonKind.String}
		if until.Valid && until.String != "" {
			if parsed, perr := time.Parse(time.RFC3339, until.String); perr == nil {
				disabled.Until = &parsed
			}
		}
		return Student{}, disabled
	}
	if status != "active" {
		return Student{}, ErrInvalidCredentials
	}
	if lockedUntil.Valid && lockedUntil.String != "" {
		if t, perr := time.Parse(time.RFC3339, lockedUntil.String); perr == nil && now.Before(t) {
			return Student{}, ErrAccountLocked
		}
	}
	if len(stored) != 64 || subtle.ConstantTimeCompare([]byte(hashPassword(password)), []byte(stored)) != 1 {
		s.recordFailedLogin(ctx, student.ID, now)
		return Student{}, ErrInvalidCredentials
	}
	if reviewStatus != "approved" {
		return Student{}, ErrRegistrationPending
	}
	s.resetFailedLogin(ctx, student.ID, now)
	return student, nil
}

func (s *Service) recordFailedLogin(ctx context.Context, studentID int64, now time.Time) {
	_, _ = s.db.ExecContext(ctx, `UPDATE student SET failed_login_count = failed_login_count + 1, updated_at = $1 WHERE id = $2`, now.UTC().Format(time.RFC3339), studentID)
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT failed_login_count FROM student WHERE id = $1`, studentID).Scan(&count); err == nil && count >= maxFailedLogins {
		_, _ = s.db.ExecContext(ctx, `UPDATE student SET locked_until = $1 WHERE id = $2`, now.Add(lockDuration).UTC().Format(time.RFC3339), studentID)
	}
}

func (s *Service) resetFailedLogin(ctx context.Context, studentID int64, now time.Time) {
	_, _ = s.db.ExecContext(ctx, `UPDATE student SET failed_login_count = 0, locked_until = NULL, last_login_at = $1 WHERE id = $2`, now.UTC().Format(time.RFC3339), studentID)
}

func (s *Service) studentExists(ctx context.Context, phone string) bool {
	if s.db == nil {
		return false
	}
	var id int64
	return s.db.QueryRowContext(ctx, `SELECT id FROM student WHERE phone = $1`, phone).Scan(&id) == nil
}

var _ *sql.DB
