package exam

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrCertNotFound = errors.New("证书不存在")

// VerifyState 验真四态（PRD §7.8 D-4）。
type VerifyState string

const (
	StateValid   VerifyState = "valid"   // 有效
	StateExpired VerifyState = "expired" // 已过期
	StateRevoked VerifyState = "revoked" // 已撤销
)

// VerifyResult 验真结果（姓名已脱敏，绝不回显完整身份证号）。
type VerifyResult struct {
	CertNo        string
	State         VerifyState
	CourseName    string
	Level         string
	StudentName   string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	RevokedReason string
}

// VerifyCert 证书验真：返回四态之一；编号不存在返回 ErrCertNotFound。
// ⚠️ 已过期的证书绝不显示为「有效」（宁严勿宽，D-4）。
func VerifyCert(ctx context.Context, db *sql.DB, certNo string, now time.Time) (VerifyResult, error) {
	var r VerifyResult
	var status, issuedRaw, expiresRaw string
	var revokedReason sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT c.cert_no, c.level, c.status, c.issued_at, c.expires_at, c.revoked_reason, co.name, st.name
		 FROM certificate c
		 JOIN course co ON co.id = c.course_id
		 JOIN student st ON st.id = c.student_id
		 WHERE c.cert_no = ?`, certNo).
		Scan(&r.CertNo, &r.Level, &status, &issuedRaw, &expiresRaw, &revokedReason, &r.CourseName, &r.StudentName)
	if err == sql.ErrNoRows {
		return VerifyResult{}, ErrCertNotFound
	}
	if err != nil {
		return VerifyResult{}, err
	}
	r.StudentName = maskName(r.StudentName)
	r.IssuedAt, _ = time.Parse(time.RFC3339, issuedRaw)
	r.ExpiresAt, _ = time.Parse(time.RFC3339, expiresRaw)
	r.RevokedReason = revokedReason.String
	switch {
	case status == "revoked":
		r.State = StateRevoked
	case r.ExpiresAt.Before(now):
		r.State = StateExpired
	default:
		r.State = StateValid
	}
	return r, nil
}

// maskName 姓名脱敏：保留首字符（姓），其余打星。
func maskName(name string) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return ""
	}
	out := string(runes[0])
	for i := 1; i < len(runes); i++ {
		out += "*"
	}
	return out
}
