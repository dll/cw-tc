package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestSessionCreateVerifyAndExpiry(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	student, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 6, 0, 0, 0, 0, time.UTC)
	session, err := service.CreateSession(context.Background(), student.ID, now)
	if err != nil || len(session.Token) != 64 {
		t.Fatalf("创建会话 = %#v, err=%v", session, err)
	}
	gotID, err := service.VerifySession(context.Background(), session.Token, now.Add(6*24*time.Hour))
	if err != nil || gotID != student.ID {
		t.Fatalf("验证会话 = %d, err=%v", gotID, err)
	}
	if _, err := service.VerifySession(context.Background(), session.Token, now.Add(7*24*time.Hour)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("过期会话错误 = %v", err)
	}
}
