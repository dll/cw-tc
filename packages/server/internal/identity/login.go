package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
)

var ErrInvalidCredentials = errors.New("手机号或密码错误")

func (s *Service) Login(ctx context.Context, phone, password string) (Student, error) {
	if s.db == nil || ValidatePhone(phone) != nil || password == "" {
		return Student{}, ErrInvalidCredentials
	}
	var student Student
	var stored []byte
	err := s.db.QueryRowContext(ctx, `SELECT id, student_no, name, phone, password_hash FROM student WHERE phone = ? AND status = 'active'`, phone).
		Scan(&student.ID, &student.StudentNo, &student.Name, &student.Phone, &stored)
	if err != nil || len(stored) != 32 {
		return Student{}, ErrInvalidCredentials
	}
	candidate := hashPassword(password)
	if subtle.ConstantTimeCompare(candidate, stored) != 1 {
		return Student{}, ErrInvalidCredentials
	}
	return student, nil
}

func (s *Service) studentExists(ctx context.Context, phone string) bool {
	if s.db == nil {
		return false
	}
	var id int64
	return s.db.QueryRowContext(ctx, `SELECT id FROM student WHERE phone = ?`, phone).Scan(&id) == nil
}

var _ *sql.DB
