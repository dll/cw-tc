// Package staff 实现内部人员账号管理（PRD §7.10 · M5-05）。
// 教师/管理员开通、停用；停用教师前必须移交课程（铁律 8）。
package staff

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrStaffInput      = errors.New("账号参数非法")
	ErrRoleInvalid     = errors.New("角色非法")
	ErrAccountExists   = errors.New("账号已存在")
	ErrStaffNotFound   = errors.New("账号不存在")
	ErrHasActiveCourse = errors.New("名下仍有课程，请先移交后再停用")
)

// Role 内部人员角色（数据字典 §2.3 staff.role）。
type Role string

const (
	RoleTeacher    Role = "teacher"
	RoleAdminOps   Role = "admin_ops"
	RoleAdminSuper Role = "admin_super"
)

func validRole(r Role) bool {
	return r == RoleTeacher || r == RoleAdminOps || r == RoleAdminSuper
}

// Create 开通教师/管理员：临时密码 + 强制首次登录改密（password_changed_at 留空）。
func Create(ctx context.Context, db *sql.DB, account, name string, role Role, password string, createdBy int64) (int64, error) {
	if db == nil || account == "" || name == "" || password == "" {
		return 0, ErrStaffInput
	}
	if !validRole(role) {
		return 0, ErrRoleInvalid
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff WHERE account = $1`, account).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrAccountExists
	}
	var createdByArg any
	if createdBy > 0 {
		createdByArg = createdBy
	}
	var id int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO staff(account, name, role, password_hash, created_source, created_by) VALUES($1,$2,$3,$4,'admin',$5) RETURNING id`,
		account, name, string(role), hashPassword(password), createdByArg).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// Disable 停用：若名下仍有课程则拒绝（必须先移交，铁律 8）。
func Disable(ctx context.Context, db *sql.DB, staffID int64) error {
	if db == nil || staffID <= 0 {
		return ErrStaffInput
	}
	var courses int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM course WHERE teacher_id = $1`, staffID).Scan(&courses); err != nil {
		return err
	}
	if courses > 0 {
		return ErrHasActiveCourse
	}
	res, err := db.ExecContext(ctx,
		`UPDATE staff SET status = 'disabled', updated_at = $1 WHERE id = $2 AND status = 'active'`,
		time.Now().UTC().Format(time.RFC3339), staffID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrStaffNotFound
	}
	return nil
}

// hashPassword 返回口令的 SHA-256 十六进制串（64 字符 ASCII）。
// 存 TEXT 列：原始 32 字节在 PostgreSQL 的 TEXT 列会因非 UTF-8 字节触发校验失败，
// 十六进制串则 SQLite / PG 均安全。
func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// ChangePassword 修改内部人员口令并标记已改密（首登强制改密闭环）。
// 旧口令校验通过才允许改；改后 password_changed_at 落当前时间。
func ChangePassword(ctx context.Context, db *sql.DB, staffID int64, oldPassword, newPassword string, now time.Time) error {
	if db == nil || staffID <= 0 || oldPassword == "" || newPassword == "" {
		return ErrStaffInput
	}
	var stored string
	if err := db.QueryRowContext(ctx, `SELECT password_hash FROM staff WHERE id = $1`, staffID).Scan(&stored); err != nil {
		return ErrStaffNotFound
	}
	if len(stored) != 64 || subtle.ConstantTimeCompare([]byte(hashPassword(oldPassword)), []byte(stored)) != 1 {
		return ErrInvalidCredentials
	}
	res, err := db.ExecContext(ctx,
		`UPDATE staff SET password_hash = $1, password_changed_at = $2, updated_at = $3 WHERE id = $4`,
		hashPassword(newPassword), now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), staffID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrStaffNotFound
	}
	return nil
}

// SeedStaff 幂等创建一个种子账号（created_source='seed'，created_by 为空，首登强制改密）。
// 已存在同名账号则跳过（created=false）；密码空串拒绝 —— **绝不设默认密码**（PRD §8.2）。
func SeedStaff(ctx context.Context, db *sql.DB, account, name string, role Role, password string) (int64, bool, error) {
	if db == nil || account == "" || password == "" {
		return 0, false, ErrStaffInput
	}
	if !validRole(role) {
		return 0, false, ErrRoleInvalid
	}
	var id int64
	err := db.QueryRowContext(ctx, `SELECT id FROM staff WHERE account = $1`, account).Scan(&id)
	if err == nil {
		return id, false, nil // 已存在，幂等跳过
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	var newID int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO staff(account, name, role, password_hash, created_source) VALUES($1,$2,$3,$4,'seed') RETURNING id`,
		account, name, string(role), hashPassword(password)).Scan(&newID); err != nil {
		return 0, false, err
	}
	return newID, true, nil
}

// SeedAdmin 幂等创建初始超管（SeedStaff 的特例）。
func SeedAdmin(ctx context.Context, db *sql.DB, account, name, password string) (int64, bool, error) {
	if name == "" {
		name = "超级管理员"
	}
	return SeedStaff(ctx, db, account, name, RoleAdminSuper, password)
}

// TransferCourse 课程移交：把课程 teacher_id 改到另一位在职教师名下（停用教师前必须，铁律 8）。
func TransferCourse(ctx context.Context, db *sql.DB, courseID, toTeacherID int64) error {
	if db == nil || courseID <= 0 || toTeacherID <= 0 {
		return ErrStaffInput
	}
	var role, status string
	if err := db.QueryRowContext(ctx, `SELECT role, status FROM staff WHERE id = $1`, toTeacherID).Scan(&role, &status); err != nil {
		return ErrStaffNotFound
	}
	if role != string(RoleTeacher) || status != "active" {
		return ErrStaffNotFound
	}
	res, err := db.ExecContext(ctx,
		`UPDATE course SET teacher_id = $1, updated_at = $2 WHERE id = $3`,
		toTeacherID, time.Now().UTC().Format(time.RFC3339), courseID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrStaffNotFound
	}
	return nil
}
