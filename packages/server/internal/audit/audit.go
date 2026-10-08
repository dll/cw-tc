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
	ActorType   string // student / staff / system
	ActorID     int64
	ActorRole   string
	Action      string // 如 order.refund / cert.revoke / user.reveal_phone
	TargetType  string
	TargetID    string
	BeforeValue string
	AfterValue  string
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
		`INSERT INTO audit_log(actor_type, actor_id, actor_role, action, target_type, target_id, before_value, after_value)
		 VALUES(?,?,?,?,?,?,?,?)`,
		in.ActorType, in.ActorID, in.ActorRole, in.Action, in.TargetType, in.TargetID, in.BeforeValue, in.AfterValue)
	return err
}
