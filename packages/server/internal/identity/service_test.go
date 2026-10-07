package identity

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestRegisterPersistsProtectedIdentityAndRejectsDuplicates(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	student, err := service.Register(context.Background(), RegisterInput{
		Name:     "样例学员",
		Phone:    "13812345678",
		IDCard:   "110101199001011237",
		Password: "not-a-real-password1",
	})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if student.ID == 0 || student.Phone != "13812345678" {
		t.Fatalf("注册结果异常: %#v", student)
	}
	var plain, hash string
	if err := database.QueryRow(`SELECT id_card_cipher, id_card_hash FROM student WHERE id = ?`, student.ID).Scan(&plain, &hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "110101199001011237") || hash == "" {
		t.Fatal("数据库不得保存身份证明文，且必须保存摘要")
	}
	_, err = service.Register(context.Background(), RegisterInput{Name: "重复", Phone: "13812345678", IDCard: "110101199001011245", Password: "another-pass-1"})
	if !errors.Is(err, ErrPhoneRegistered) {
		t.Fatalf("重复手机号错误 = %v，期望 ErrPhoneRegistered", err)
	}
}

func TestRegisterRequiresConfiguredDatabaseAndValidInput(t *testing.T) {
	service := NewService(nil, make([]byte, 32), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	_, err := service.Register(context.Background(), RegisterInput{Name: "样例", Phone: "bad"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("非法输入错误 = %v", err)
	}
	_, err = NewService((*sql.DB)(nil), make([]byte, 32), make([]byte, 32)).Register(context.Background(), RegisterInput{Name: "样例", Phone: "13812345678", IDCard: "110101199001011237", Password: "password1"})
	if !errors.Is(err, ErrDatabaseNotConfigured) {
		t.Fatalf("未配置数据库错误 = %v", err)
	}
}
