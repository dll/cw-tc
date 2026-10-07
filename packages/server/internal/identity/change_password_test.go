package identity

import (
	"context"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestChangePasswordVerifiesOldPasswordAndInvalidatesIt(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwTC-tests-012345678901234"))
	student, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "old-pass-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ChangePassword(context.Background(), student.ID, "wrong-pass", "new-pass-1"); err != ErrInvalidCredentials {
		t.Fatalf("旧密码错误应拒绝: %v", err)
	}
	if err := service.ChangePassword(context.Background(), student.ID, "old-pass-1", "new-pass-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "old-pass-1"); err != ErrInvalidCredentials {
		t.Fatalf("旧密码应已失效: %v", err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "new-pass-1"); err != nil {
		t.Fatalf("新密码应可登录: %v", err)
	}
}
