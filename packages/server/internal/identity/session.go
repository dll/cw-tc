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

// DeviceSession 是「账号与安全」里的登录设备记录。
type DeviceSession struct {
	ID        int64
	UserAgent string
	IP        string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Current   bool
}

func (s *Service) CreateSession(ctx context.Context, studentID int64, now time.Time) (Session, error) {
	return s.CreateSessionWithDevice(ctx, studentID, "", "", now)
}

// CreateSessionWithDevice 记录 UA / IP，供学员在「账号与安全」查看登录设备。
func (s *Service) CreateSessionWithDevice(ctx context.Context, studentID int64, userAgent, ip string, now time.Time) (Session, error) {
	if s.db == nil || studentID <= 0 {
		return Session{}, ErrSessionNotFound
	}
	// 内部测试/导入可创建会话；HTTP 登录路径已经在 Login 中强制审核状态。
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("生成会话令牌失败: %w", err)
	}
	token := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	expires := now.Add(7 * 24 * time.Hour)
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO student_session(student_id, token_hash, issued_at, expires_at, user_agent, ip) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		studentID, digest[:], now.UTC().Format(time.RFC3339), expires.UTC().Format(time.RFC3339), emptyToNil(userAgent), emptyToNil(ip)).Scan(&id)
	if err != nil {
		return Session{}, fmt.Errorf("创建会话失败: %w", err)
	}
	return Session{ID: id, Token: token, ExpiresAt: expires}, nil
}

func emptyToNil(value string) any {
	if value == "" || value == "unknown" {
		return nil
	}
	return value
}

// ListSessions 列出未撤销、未过期的登录设备，并标记当前会话。
func (s *Service) ListSessions(ctx context.Context, studentID int64, currentToken string, now time.Time) ([]DeviceSession, error) {
	if s.db == nil || studentID <= 0 {
		return nil, ErrSessionNotFound
	}
	var currentID int64
	if currentToken != "" {
		digest := sha256.Sum256([]byte(currentToken))
		_ = s.db.QueryRowContext(ctx,
			`SELECT id FROM student_session WHERE token_hash = $1 AND revoked_at IS NULL`, digest[:]).Scan(&currentID)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, COALESCE(user_agent,''), COALESCE(CAST(ip AS TEXT),''), issued_at, expires_at
		 FROM student_session
		 WHERE student_id = $1 AND revoked_at IS NULL AND expires_at > $2
		 ORDER BY issued_at DESC, id DESC`,
		studentID, now.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DeviceSession, 0)
	for rows.Next() {
		var item DeviceSession
		var issued, expires string
		if err := rows.Scan(&item.ID, &item.UserAgent, &item.IP, &issued, &expires); err != nil {
			return nil, err
		}
		item.IssuedAt, _ = time.Parse(time.RFC3339, issued)
		item.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
		item.Current = item.ID == currentID
		items = append(items, item)
	}
	return items, rows.Err()
}

// RevokeSessionByID 只允许撤销本人的会话。
func (s *Service) RevokeSessionByID(ctx context.Context, studentID, sessionID int64) error {
	if s.db == nil || studentID <= 0 || sessionID <= 0 {
		return ErrSessionNotFound
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE student_session SET revoked_at = CURRENT_TIMESTAMP WHERE id = $1 AND student_id = $2 AND revoked_at IS NULL`,
		sessionID, studentID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrSessionNotFound
	}
	return nil
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
