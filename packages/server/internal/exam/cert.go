package exam

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidCertLevel = errors.New("证书档位非法")
	ErrCertIssueInput   = errors.New("发证参数非法")
)

// certSeqQuerier 抽象 QueryRowContext，兼容 *sql.DB 与 *sql.Tx。
type certSeqQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// levelDigit 档位 → 编号档位字母（数据字典 §5.5）：L1→A、L2→M、L3→H。
func levelDigit(level string) (string, error) {
	switch level {
	case "L1":
		return "A", nil
	case "L2":
		return "M", nil
	case "L3":
		return "H", nil
	}
	return "", ErrInvalidCertLevel
}

// certYear 取 Asia/Shanghai 年份（China Standard Time，无夏令时）。
func certYear(now time.Time) int {
	return now.In(time.FixedZone("CST", 8*3600)).Year()
}

// AllocateCertNo 原子取号（cert_no_sequence），返回唯一证书编号。
// 编号格式 `CW-CN-{levelDigit}{year}{seq:05d}`，如 `CW-CN-M202600137`。
// 用 `INSERT ... ON CONFLICT ... DO UPDATE ... RETURNING` 保证并发下唯一且不重号。
func AllocateCertNo(ctx context.Context, q certSeqQuerier, courseCode, level string) (string, error) {
	digit, err := levelDigit(level)
	if err != nil {
		return "", err
	}
	year := certYear(time.Now().UTC())
	var seq int
	if err := q.QueryRowContext(ctx,
		`INSERT INTO cert_no_sequence(course_code, level_digit, year, last_seq)
		 VALUES(?, ?, ?, 1)
		 ON CONFLICT(course_code, level_digit, year) DO UPDATE SET last_seq = cert_no_sequence.last_seq + 1
		 RETURNING last_seq`, courseCode, digit, year).Scan(&seq); err != nil {
		return "", err
	}
	return fmt.Sprintf("CW-CN-%s%d%05d", digit, year, seq), nil
}

// CertIssueInput 发证入参。
type CertIssueInput struct {
	StudentID  int64
	CourseID   int64
	CourseCode string
	Level      string
	Score      float64
	Band       string
}

// IssueCertificate 发证：取号 + 写入 certificate（同一事务，失败不消耗号，铁律 4）。
func IssueCertificate(ctx context.Context, db *sql.DB, in CertIssueInput) (string, error) {
	if in.StudentID <= 0 || in.CourseID <= 0 || in.CourseCode == "" || in.Score < 0 || in.Band == "" {
		return "", ErrCertIssueInput
	}
	if _, err := levelDigit(in.Level); err != nil {
		return "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	certNo, err := AllocateCertNo(ctx, tx, in.CourseCode, in.Level)
	if err != nil {
		return "", err
	}
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.AddDate(3, 0, 0) // 有效期 3 年（数据字典 §5.4）
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		certNo, in.StudentID, in.CourseID, in.Level, in.Score, in.Band,
		issuedAt.Format(time.RFC3339), expiresAt.Format(time.RFC3339)); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return certNo, nil
}
