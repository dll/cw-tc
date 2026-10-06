package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestAuthenticateRejectsDisabledStudent(t *testing.T) {
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
	now := time.Now().UTC()
	session, err := service.CreateSession(context.Background(), student.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE student SET status = 'suspended' WHERE id = ?`, student.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), session.Token, now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("停用账号认证错误 = %v", err)
	}
}
