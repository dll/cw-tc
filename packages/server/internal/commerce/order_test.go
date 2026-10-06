package commerce

import (
	"context"
	"testing"
	"time"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func TestCreateOrderIsIdempotentForPendingCourse(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	// Minimal FK parents for isolated order test.
	_, _ = database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',X'01',X'02','[]')`)
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'sample','样例管理员','admin_ops',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','样例课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	service := NewService(database)
	input := OrderInput{StudentID: 1, CourseID: 1, RequestID: "req-1", CourseName: "样例课程", PriceCents: 1000, ExpiresAt: time.Now().Add(30 * time.Minute)}
	one, err := service.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	two, err := service.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if one.OrderNo != two.OrderNo {
		t.Fatalf("待支付订单未复用: %s != %s", one.OrderNo, two.OrderNo)
	}
}
