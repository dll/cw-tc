package identity

import (
	"context"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestAuthenticateAcceptsValidSessionAndRejectsRevoked(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 6, 0, 0, 0, 0, time.UTC)
	session, err := service.CreateSession(context.Background(), student.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.Authenticate(context.Background(), session.Token, now)
	if err != nil || got.ID != student.ID || got.Phone != student.Phone {
		t.Fatalf("认证结果 = %#v, err=%v", got, err)
	}
}
