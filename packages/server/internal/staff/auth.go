package staff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("账号或密码错误")
	ErrAccountDisabled    = errors.New("账号已停用")
	ErrUnauthenticated    = errors.New("未认证")
)

// Staff 内部人员身份（登录后）。
type Staff struct {
	ID                 int64
	Account            string
	Name               string
	Role               Role
	MustChangePassword bool // 仍是初始密码（password_changed_at 为空）→ 首登强制改密
}

// Login 内部人员登录：校验口令（常数时间比较）+ 发会话。
func Login(ctx context.Context, db *sql.DB, account, password string, now time.Time) (Staff, string, error) {
	if db == nil || account == "" || password == "" {
		return Staff{}, "", ErrInvalidCredentials
	}
	var st Staff
	var roleStr, status string
	var stored []byte
	var changedAt sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT id, account, name, role, password_hash, status, password_changed_at FROM staff WHERE account = ?`, account).
		Scan(&st.ID, &st.Account, &st.Name, &roleStr, &stored, &status, &changedAt); err != nil {
		return Staff{}, "", ErrInvalidCredentials
	}
	if status != "active" {
		return Staff{}, "", ErrAccountDisabled
	}
	if len(stored) != sha256.Size || subtle.ConstantTimeCompare(hashPassword(password), stored) != 1 {
		return Staff{}, "", ErrInvalidCredentials
	}
	st.Role = Role(roleStr)
	st.MustChangePassword = !changedAt.Valid || changedAt.String == ""
	token, err := createSession(ctx, db, st.ID, now)
	if err != nil {
		return Staff{}, "", err
	}
	return st, token, nil
}

// Authenticate 校验会话 → 返回内部人员（含角色）。
func Authenticate(ctx context.Context, db *sql.DB, token string, now time.Time) (Staff, error) {
	if db == nil || token == "" {
		return Staff{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	var staffID int64
	var expiresAt string
	if err := db.QueryRowContext(ctx,
		`SELECT staff_id, expires_at FROM staff_session WHERE token_hash = ? AND revoked_at IS NULL`, digest[:]).
		Scan(&staffID, &expiresAt); err != nil {
		return Staff{}, ErrUnauthenticated
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || !now.Before(expires) {
		return Staff{}, ErrUnauthenticated
	}
	var st Staff
	var roleStr string
	if err := db.QueryRowContext(ctx,
		`SELECT id, account, name, role FROM staff WHERE id = ? AND status = 'active'`, staffID).
		Scan(&st.ID, &st.Account, &st.Name, &roleStr); err != nil {
		return Staff{}, ErrUnauthenticated
	}
	st.Role = Role(roleStr)
	return st, nil
}

func createSession(ctx context.Context, db *sql.DB, staffID int64, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	expires := now.Add(12 * time.Hour) // 后台会话 12 小时
	_, err := db.ExecContext(ctx,
		`INSERT INTO staff_session(staff_id, token_hash, issued_at, expires_at) VALUES(?,?,?,?)`,
		staffID, digest[:], now.UTC().Format(time.RFC3339), expires.UTC().Format(time.RFC3339))
	return token, err
}
