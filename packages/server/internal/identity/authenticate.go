package identity

import (
	"context"
	"errors"
	"time"
)

var ErrUnauthenticated = errors.New("未认证")

func (s *Service) Authenticate(ctx context.Context, token string, now time.Time) (Student, error) {
	studentID, err := s.VerifySession(ctx, token, now)
	if err != nil {
		return Student{}, ErrUnauthenticated
	}
	var student Student
	err = s.db.QueryRowContext(ctx, `SELECT id, student_no, name, phone FROM student WHERE id = $1 AND status = 'active'`, studentID).
		Scan(&student.ID, &student.StudentNo, &student.Name, &student.Phone)
	if err != nil {
		return Student{}, ErrUnauthenticated
	}
	return student, nil
}
