package staff

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

func TestTransferCourse(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()

	teacherA, _ := Create(ctx, database, "ta", "甲老师", RoleTeacher, "Temp@123", 1)
	teacherB, _ := Create(ctx, database, "tb", "乙老师", RoleTeacher, "Temp@123", 1)
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程','main',1,'published',?)`, teacherA); err != nil {
		t.Fatal(err)
	}
	// 移交给乙老师
	if err := TransferCourse(ctx, database, 1, teacherB); err != nil {
		t.Fatal(err)
	}
	var owner int64
	_ = database.QueryRow(`SELECT teacher_id FROM course WHERE id=1`).Scan(&owner)
	if owner != teacherB {
		t.Fatalf("课程应移交给乙老师(%d)，实际 %d", teacherB, owner)
	}
	// 移交给非教师（超管）→ 拒绝
	if err := TransferCourse(ctx, database, 1, 1); err != ErrStaffNotFound {
		t.Fatalf("移交给非教师应报 ErrStaffNotFound，实际 %v", err)
	}
}

func TestSeedAdminIdempotent(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()

	// 首次创建
	id, created, err := SeedAdmin(ctx, database, "boss", "", "初始密码-1")
	if err != nil || !created {
		t.Fatalf("首次种子应 created=true，实际 created=%v err=%v", created, err)
	}
	if id <= 0 {
		t.Fatalf("种子 id 非法: %d", id)
	}
	var role string
	_ = database.QueryRow(`SELECT role FROM staff WHERE account='boss'`).Scan(&role)
	if role != string(RoleAdminSuper) {
		t.Fatalf("种子角色应为 admin_super，实际 %s", role)
	}
	// 幂等：再次种子跳过
	id2, created2, err := SeedAdmin(ctx, database, "boss", "", "另一密码")
	if err != nil || created2 || id2 != id {
		t.Fatalf("二次种子应幂等跳过: created=%v id2=%d(想 %d) err=%v", created2, id2, id, err)
	}
	// 空密码拒绝
	if _, _, err := SeedAdmin(ctx, database, "boss2", "", ""); err != ErrStaffInput {
		t.Fatalf("空密码应报 ErrStaffInput，实际 %v", err)
	}
	// 能登录（说明密码哈希一致）
	if _, _, err := Login(ctx, database, "boss", "初始密码-1", time.Now().UTC()); err != nil {
		t.Fatalf("种子账号应可登录: %v", err)
	}
}

func TestChangePasswordAndMustChangeFlag(t *testing.T) {
	database := staffDB(t)
	defer database.Close()
	ctx := context.Background()
	id, created, err := SeedAdmin(ctx, database, "boss", "", "初始密码-1")
	if err != nil || !created {
		t.Fatalf("种子失败: created=%v err=%v", created, err)
	}
	// 首登应标记 must_change
	st, _, err := Login(ctx, database, "boss", "初始密码-1", time.Now().UTC())
	if err != nil || !st.MustChangePassword {
		t.Fatalf("首登应 must_change=true，实际 %v err=%v", st.MustChangePassword, err)
	}
	// 改密
	if err := ChangePassword(ctx, database, id, "初始密码-1", "新密码-2", time.Now().UTC()); err != nil {
		t.Fatalf("改密失败: %v", err)
	}
	// 旧密码失效，新密码可登录且不再强制改密
	if _, _, err := Login(ctx, database, "boss", "初始密码-1", time.Now().UTC()); err == nil {
		t.Fatalf("旧密码应失效")
	}
	st2, _, err := Login(ctx, database, "boss", "新密码-2", time.Now().UTC())
	if err != nil || st2.MustChangePassword {
		t.Fatalf("改密后应可登录且 must_change=false，实际 %v err=%v", st2.MustChangePassword, err)
	}
	// 旧密码错误改密 → ErrInvalidCredentials
	if err := ChangePassword(ctx, database, id, "错误旧密码", "x", time.Now().UTC()); err != ErrInvalidCredentials {
		t.Fatalf("旧密码错误应报 ErrInvalidCredentials，实际 %v", err)
	}
}
