// Package staff 实现内部人员账号管理（PRD §7.10 · M5-05）。
// 教师/管理员开通、停用；停用教师前必须移交课程（铁律 8）。
package staff

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff WHERE account = ?`, account).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrAccountExists
	}
	var createdByArg any
	if createdBy > 0 {
		createdByArg = createdBy
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO staff(account, name, role, password_hash, created_source, created_by) VALUES(?,?,?,?,'admin',?)`,
		account, name, string(role), hashPassword(password), createdByArg)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Disable 停用：若名下仍有课程则拒绝（必须先移交，铁律 8）。
func Disable(ctx context.Context, db *sql.DB, staffID int64) error {
	if db == nil || staffID <= 0 {
		return ErrStaffInput
	}
	var courses int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM course WHERE teacher_id = ?`, staffID).Scan(&courses); err != nil {
		return err
	}
	if courses > 0 {
		return ErrHasActiveCourse
	}
	res, err := db.ExecContext(ctx,
		`UPDATE staff SET status = 'disabled', updated_at = ? WHERE id = ? AND status = 'active'`,
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

// hashPassword 与 identity 一致的散列（内部人员临时口令，非密码学强度要求场景）。
func hashPassword(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return sum[:]
}
