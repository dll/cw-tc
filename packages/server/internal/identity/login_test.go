package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestLoginFindsStudentByPhoneAndRejectsWrongPassword(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	if _, err := service.Register(context.Background(), RegisterInput{Name: "样例学员", Phone: "13812345678", IDCard: "110101199001011237", Password: "password-1"}); err != nil {
		t.Fatal(err)
	}
	student, err := service.Login(context.Background(), "13812345678", "password-1")
	if err != nil || student.Phone != "13812345678" {
		t.Fatalf("登录结果 = %#v, err=%v", student, err)
	}
	if _, err := service.Login(context.Background(), "13812345678", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("错误密码 = %v，期望 ErrInvalidCredentials", err)
	}
}

func TestLoginDoesNotRevealUnknownPhone(t *testing.T) {
	service := NewService(nil, nil, nil)
	if _, err := service.Login(context.Background(), "13812345678", "password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("未知手机号必须统一返回认证失败，实际 %v", err)
	}
}
