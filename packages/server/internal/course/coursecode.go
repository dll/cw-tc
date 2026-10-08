// Package course 实现平台课程管理（PRD §7.9 J · M5-02）。
package course

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
)

var ErrCodeAlloc = errors.New("课程码分配失败")

// alphabet 去掉易混淆字符（0/O、1/I/L）的字母数字集。
const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// AllocateCourseCode 分配全局唯一课程码（TC-XXXX，PRD §7.9 J-1），不与现有课程码（含上游）冲突。
func AllocateCourseCode(ctx context.Context, db *sql.DB) (string, error) {
	if db == nil {
		return "", ErrCodeAlloc
	}
	for i := 0; i < 20; i++ {
		code := "TC-" + randomSuffix(4)
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM course WHERE course_code = $1`, code).Scan(&n); err != nil {
			return "", err
		}
		if n == 0 {
			return code, nil
		}
	}
	return "", ErrCodeAlloc
}

func randomSuffix(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
