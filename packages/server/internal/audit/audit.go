// Package audit 实现审计留痕（PRD §7.10.9 · 数据字典 §7.3）。
// 高危操作（退款/撤证/改价/下架/停用/授权/看全号）必须写审计；只增不改不删。
package audit

import (
	"context"
	"database/sql"
	"errors"
)

var ErrAuditInput = errors.New("审计参数非法")

// Input 审计记录入参。
type Input struct {
	ActorType  string // student / staff / system
	ActorID    int64
	ActorRole  string
	Action     string // 如 order.refund / cert.revoke / user.reveal_phone
	TargetType string
	TargetID   string
	Before     string // 原值 JSON（改价/改状态必填；空串存 NULL）
	After      string // 新值 JSON（空串存 NULL）
	IP         string // 操作来源 IP（空串存 NULL）
}

// Record 写入一条审计记录。
func Record(ctx context.Context, db *sql.DB, in Input) error {
	if db == nil || in.Action == "" || in.ActorID <= 0 {
		return ErrAuditInput
	}
	if in.ActorType != "student" && in.ActorType != "staff" && in.ActorType != "system" {
		return ErrAuditInput
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO audit_log(actor_type, actor_id, actor_role, action, target_type, target_id, before, after, ip)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		in.ActorType, in.ActorID, in.ActorRole, in.Action, in.TargetType, in.TargetID,
		nullIfEmpty(in.Before), nullIfEmpty(in.After), nullIfEmpty(in.IP))
	return err
}

// nullIfEmpty 空串映射为 NULL：PostgreSQL 的 JSONB / INET 不接受空串。
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
