package course

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func courseDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'t','教师','teacher',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestAllocateCourseCode(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 分配格式 TC-XXXX
	code, err := AllocateCourseCode(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "TC-") || len(code) != 7 {
		t.Fatalf("课程码格式应为 TC-XXXX，实际 %s", code)
	}
	// 已占用该码后再分配，不应冲突
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,$1,'platform','课程','main',1,'published',1)`, code); err != nil {
		t.Fatal(err)
	}
	code2, err := AllocateCourseCode(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if code2 == code {
		t.Fatalf("课程码不应重复: %s", code)
	}
	// 与上游课程码不冲突（喂一个 TC- 形式的上游码）
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(2,'03-CF','upstream','上游课','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := AllocateCourseCode(ctx, database); err != nil {
		t.Fatalf("与上游码并存时分配失败: %v", err)
	}
}
