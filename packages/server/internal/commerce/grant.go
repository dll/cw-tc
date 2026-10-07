package commerce

import (
	"context"
	"errors"
	"time"
)

var (
	ErrGrantInput          = errors.New("授权参数非法")
	ErrEntitlementExists   = errors.New("该学员本课已有有效权益")
	ErrEntitlementNotFound = errors.New("无有效权益可撤销")
)

// GrantEntitlement 管理员授权免费学习（铁律 8 · §7.3 G-7）：source='grant'，授权人与事由必填。
// 一门课同一时刻只允许一条有效权益（DB 部分唯一索引 uq_entitlement_active 兜底）。
func (s *Service) GrantEntitlement(ctx context.Context, studentID, courseID, grantedBy int64, grantReason string) error {
	if s.db == nil || studentID <= 0 || courseID <= 0 || grantedBy <= 0 || grantReason == "" {
		return ErrGrantInput
	}
	var active int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE student_id = ? AND course_id = ? AND revoked_at IS NULL`,
		studentID, courseID).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return ErrEntitlementExists
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO entitlement(student_id, course_id, source, granted_by, grant_reason, granted_at) VALUES(?,?,'grant',?,?,?)`,
		studentID, courseID, grantedBy, grantReason, time.Now().UTC().Format(time.RFC3339))
	return err
}

// RevokeGrant 撤销授权：只收权益、已发证保留（铁律 8：撤销授权 ≠ 退款；撤证只走 normal 退款）。
func (s *Service) RevokeGrant(ctx context.Context, studentID, courseID int64) error {
	if s.db == nil || studentID <= 0 || courseID <= 0 {
		return ErrGrantInput
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE entitlement SET revoked_at = ?, revoke_reason = 'admin' WHERE student_id = ? AND course_id = ? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), studentID, courseID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrEntitlementNotFound
	}
	return nil
}
