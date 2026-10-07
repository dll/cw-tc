package staff

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func staffDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'root','超管','admin_super',X'01','seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id, name) VALUES(1,'样例专业')`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestCreateStaffAndDisable(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()

	// 开通教师
	id, err := Create(ctx, database, "teacher01", "张老师", RoleTeacher, "Temp@123", 1)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Fatalf("开通失败 id=%d", id)
	}
	// 重复账号 → 拒绝
	if _, err := Create(ctx, database, "teacher01", "张老师", RoleTeacher, "Temp@123", 1); err != ErrAccountExists {
		t.Fatalf("重复账号应报 ErrAccountExists，实际 %v", err)
	}
	// 非法角色 → 拒绝
	if _, err := Create(ctx, database, "x", "y", Role("boss"), "z", 1); err != ErrRoleInvalid {
		t.Fatalf("非法角色应报 ErrRoleInvalid，实际 %v", err)
	}
	// 停用（无课程）成功
	if err := Disable(ctx, database, id); err != nil {
		t.Fatal(err)
	}
	// 已停用再停用 → ErrStaffNotFound
	if err := Disable(ctx, database, id); err != ErrStaffNotFound {
		t.Fatalf("已停用再停用应报 ErrStaffNotFound，实际 %v", err)
	}
}

func TestDisableTeacherWithCourseRejected(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()

	id, err := Create(ctx, database, "teacher02", "李老师", RoleTeacher, "Temp@123", 1)
	if err != nil {
		t.Fatal(err)
	}
	// 该教师名下有一门课
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程','main',1,'published',?)`, id); err != nil {
		t.Fatal(err)
	}
	// 停用应被拒（须先移交课程）
	if err := Disable(ctx, database, id); err != ErrHasActiveCourse {
		t.Fatalf("名下课程未移交应报 ErrHasActiveCourse，实际 %v", err)
	}
}
