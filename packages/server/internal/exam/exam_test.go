package exam

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func examDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	encKey := []byte("01234567890123456789012345678901")
	hmacKey := []byte("hmac-key-for-cwtc-tests-012345678901234")
	ciphertext, hash, err := identity.ProtectIDCard("110101199001011237", encKey, hmacKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO student(id, student_no, name, phone, id_card_hash, id_card_cipher, agreement_versions) VALUES(1,'S00000001','样例学员','13812345678',$1,$2,'[]')`, hash, ciphertext); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','样例课程','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	// 3 道 L1 题（2 客观 + 1 主观）
	for _, q := range []struct{ qno, typ, grad, stem, ans string }{
		{"L1-01-01", "single", "auto", "赋值是表达式吗？", "A"},
		{"L1-01-02", "judge", "auto", "指针是地址。", "对"},
		{"L1-02-01", "short", "manual", "简述指针作用。", "间接访问内存"},
	} {
		if _, err := database.Exec(`INSERT INTO question(course_id, qno, level, type, gradability, stem, answer, score) VALUES(1,$1,'L1',$2,$3,$4,$5,1.5)`,
			q.qno, q.typ, q.grad, q.stem, q.ans); err != nil {
			t.Fatal(err)
		}
	}
	return database
}

func grantEntitlement(t *testing.T, database *sql.DB, studentID, courseID int64) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO entitlement(student_id, course_id, source, granted_at) VALUES($1,$2,'free','2026-01-01T00:00:00Z')`, studentID, courseID); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPaperOmitsAnswer(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	paper, err := BuildPaper(context.Background(), database, 1, "L1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(paper, "间接访问内存") || strings.Contains(paper, `"answer"`) || strings.Contains(paper, `"analysis"`) {
		t.Fatalf("题单泄露答案/解析: %s", paper)
	}
	var items []PaperItem
	if err := json.Unmarshal([]byte(paper), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("应组 3 题，实际 %d", len(items))
	}
}

func TestBuildPaperLimitAndEmpty(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	paper, err := BuildPaper(context.Background(), database, 1, "L1", 2)
	if err != nil {
		t.Fatal(err)
	}
	var items []PaperItem
	_ = json.Unmarshal([]byte(paper), &items)
	if len(items) != 2 {
		t.Fatalf("limit=2 应抽 2 题，实际 %d", len(items))
	}
	if _, err := BuildPaper(context.Background(), database, 1, "L3", 0); err != ErrEmptyPaper {
		t.Fatalf("空档位应报 ErrEmptyPaper，实际 %v", err)
	}
}

func TestStartSessionRequiresEntitlementAndSingleSession(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	paper, err := BuildPaper(context.Background(), database, 1, "L1", 0)
	if err != nil {
		t.Fatal(err)
	}
	// 未解锁 → 拒绝
	if _, err := StartSession(context.Background(), database, 1, 1, "L1", paper); err != ErrNoEntitlement {
		t.Fatalf("未解锁应报 ErrNoEntitlement，实际 %v", err)
	}
	// 解锁后开考成功
	grantEntitlement(t, database, 1, 1)
	id, err := StartSession(context.Background(), database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Fatalf("开考失败: id=%d", id)
	}
	// 同档再开 → 单档单会话拒绝
	if _, err := StartSession(context.Background(), database, 1, 1, "L1", paper); err != ErrSessionOngoing {
		t.Fatalf("同档二会话应报 ErrSessionOngoing，实际 %v", err)
	}
	// 换档位（L2）不受影响
	if _, err := StartSession(context.Background(), database, 1, 1, "L2", paper); err != nil {
		t.Fatalf("换档位开考不应受影响: %v", err)
	}
}
