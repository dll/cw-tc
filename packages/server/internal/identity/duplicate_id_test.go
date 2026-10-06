package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestRegisterRejectsDuplicateIDCardAcrossPhones(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	input := RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"}
	if _, err := service.Register(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	input.Phone = "13912345678"
	if _, err := service.Register(context.Background(), input); !errors.Is(err, ErrIDCardRegistered) {
		t.Fatalf("重复身份证错误 = %v，期望 ErrIDCardRegistered", err)
	}
}
