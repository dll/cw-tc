package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrSessionNotFound = errors.New("会话不存在或已失效")

type Session struct {
	ID        int64
	Token     string
	ExpiresAt time.Time
}

func (s *Service) CreateSession(ctx context.Context, studentID int64, now time.Time) (Session, error) {
	if s.db == nil || studentID <= 0 {
		return Session{}, ErrSessionNotFound
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("生成会话令牌失败: %w", err)
	}
	token := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	expires := now.Add(7 * 24 * time.Hour)
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO student_session(student_id, token_hash, issued_at, expires_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		studentID, digest[:], now.UTC().Format(time.RFC3339), expires.UTC().Format(time.RFC3339)).Scan(&id)
	if err != nil {
		return Session{}, fmt.Errorf("创建会话失败: %w", err)
	}
	return Session{ID: id, Token: token, ExpiresAt: expires}, nil
}

func (s *Service) RevokeAllSessions(ctx context.Context, studentID int64) error {
	if s.db == nil || studentID <= 0 {
		return ErrSessionNotFound
	}
	_, err := s.db.ExecContext(ctx, `UPDATE student_session SET revoked_at = CURRENT_TIMESTAMP WHERE student_id = $1 AND revoked_at IS NULL`, studentID)
	return err
}

func (s *Service) RevokeSession(ctx context.Context, token string) error {
	if s.db == nil || token == "" {
		return ErrSessionNotFound
	}
	digest := sha256.Sum256([]byte(token))
	result, err := s.db.ExecContext(ctx, `UPDATE student_session SET revoked_at = CURRENT_TIMESTAMP WHERE token_hash = $1 AND revoked_at IS NULL`, digest[:])
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (s *Service) VerifySession(ctx context.Context, token string, now time.Time) (int64, error) {
	if s.db == nil || token == "" {
		return 0, ErrSessionNotFound
	}
	digest := sha256.Sum256([]byte(token))
	var studentID int64
	var expiresAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT student_id, expires_at FROM student_session WHERE token_hash = $1 AND revoked_at IS NULL`, digest[:]).Scan(&studentID, &expiresAt)
	if err != nil {
		return 0, ErrSessionNotFound
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || !now.Before(expires) {
		return 0, ErrSessionNotFound
	}
	return studentID, nil
}
