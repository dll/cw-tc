package exam

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestVerifyCertFourStates(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	insert := func(certNo, level, status, expiresAt string) {
		if _, err := database.Exec(`INSERT INTO certificate(cert_no, student_id, course_id, level, score, band, issued_at, expires_at, status) VALUES(?,1,1,?,88,'good','2026-01-01T00:00:00Z',?,'valid')`, certNo, level, expiresAt); err != nil {
			t.Fatal(err)
		}
		if status == "revoked" {
			if _, err := database.Exec(`UPDATE certificate SET status='revoked', revoked_reason='admin' WHERE cert_no=?`, certNo); err != nil {
				t.Fatal(err)
			}
		}
	}

	insert("CW-CN-M000000001", "L2", "valid", now.AddDate(1, 0, 0).Format(time.RFC3339))
	insert("CW-CN-M000000002", "L3", "expired", now.AddDate(-1, 0, 0).Format(time.RFC3339))
	insert("CW-CN-M000000003", "L1", "revoked", now.AddDate(1, 0, 0).Format(time.RFC3339))

	// 有效
	r, err := VerifyCert(ctx, database, "CW-CN-M000000001", now)
	if err != nil || r.State != StateValid {
		t.Fatalf("应有效: state=%v err=%v", r.State, err)
	}
	if r.StudentName != "样***" {
		t.Fatalf("姓名应脱敏为 样***，实际 %s", r.StudentName)
	}
	// 已过期
	r, _ = VerifyCert(ctx, database, "CW-CN-M000000002", now)
	if r.State != StateExpired {
		t.Fatalf("应已过期，实际 %v", r.State)
	}
	// 已撤销
	r, _ = VerifyCert(ctx, database, "CW-CN-M000000003", now)
	if r.State != StateRevoked || r.RevokedReason != "admin" {
		t.Fatalf("应已撤销: state=%v reason=%s", r.State, r.RevokedReason)
	}
	// 不存在
	if _, err := VerifyCert(ctx, database, "CW-CN-M000000099", now); !errors.Is(err, ErrCertNotFound) {
		t.Fatalf("不存在应报 ErrCertNotFound，实际 %v", err)
	}
}
