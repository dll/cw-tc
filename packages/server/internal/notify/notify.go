// Package notify 实现通知与广播（PRD §7.10 M-2 · M5-09）。
// 广播属高危操作：创建草稿 → 二次确认才发送（铁律 8）。
package notify

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrBroadcastInput       = errors.New("广播参数非法")
	ErrTargetInvalid        = errors.New("目标类型非法")
	ErrBroadcastNotFound    = errors.New("广播不存在")
	ErrBroadcastForbidden   = errors.New("无权操作该广播")
	ErrNotificationNotFound = errors.New("通知不存在")
)

// BroadcastInput 广播草稿入参。
type BroadcastInput struct {
	OperatorID     int64
	Target         string // all / role / course / major
	TargetValue    string
	Title          string
	Body           string
	Link           string
	IdempotencyKey string
}

func validTarget(t string) bool {
	return t == "all" || t == "role" || t == "course" || t == "major"
}

// CreateBroadcast 创建广播草稿（第一步，不发送）。幂等键重复时返回已有草稿。
func CreateBroadcast(ctx context.Context, db *sql.DB, in BroadcastInput) (int64, error) {
	if db == nil || in.OperatorID <= 0 || in.Title == "" || in.Body == "" || in.IdempotencyKey == "" || !validTarget(in.Target) {
		return 0, ErrBroadcastInput
	}
	var existing sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT id FROM broadcast WHERE idempotency_key = $1`, in.IdempotencyKey).Scan(&existing); err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if existing.Valid {
		return existing.Int64, nil
	}
	var id int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO broadcast(operator_id, target, target_value, title, body, link, idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.OperatorID, in.Target, in.TargetValue, in.Title, in.Body, in.Link, in.IdempotencyKey).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// ConfirmBroadcast 二次确认发送（第二步）：幂等 —— 已发送则直接返回实际人数，不重复扩散。
func ConfirmBroadcast(ctx context.Context, db *sql.DB, broadcastID, operatorID int64) (int, error) {
	if db == nil || broadcastID <= 0 || operatorID <= 0 {
		return 0, ErrBroadcastInput
	}
	var op int64
	var target, targetValue, title, body, link string
	var sentAt sql.NullString
	var recipientsCount int
	if err := db.QueryRowContext(ctx,
		`SELECT operator_id, target, target_value, title, body, link, sent_at, recipients_count FROM broadcast WHERE id = $1`,
		broadcastID).Scan(&op, &target, &targetValue, &title, &body, &link, &sentAt, &recipientsCount); err != nil {
		if err == sql.ErrNoRows {
			return 0, ErrBroadcastNotFound
		}
		return 0, err
	}
	if op != operatorID {
		return 0, ErrBroadcastForbidden
	}
	if sentAt.Valid { // 幂等：已发送（sent_at 非空 = 已确认发送）
		return recipientsCount, nil
	}
	ids, err := recipients(ctx, db, target, targetValue)
	if err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, sid := range ids {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notification(student_id, type, title, body, link, broadcast_id) VALUES($1,'broadcast',$2,$3,$4,$5)`,
			sid, title, body, link, broadcastID); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE broadcast SET sent_at=$1, recipients_count=$2 WHERE id=$3`,
		time.Now().UTC().Format(time.RFC3339), len(ids), broadcastID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// recipients 按广播目标计算收件学员（去重）。
func recipients(ctx context.Context, db *sql.DB, target, targetValue string) ([]int64, error) {
	var rows *sql.Rows
	var err error
	switch target {
	case "all", "role":
		rows, err = db.QueryContext(ctx, `SELECT id FROM student`)
	case "course":
		rows, err = db.QueryContext(ctx,
			`SELECT DISTINCT e.student_id FROM entitlement e JOIN course c ON e.course_id = c.id WHERE c.course_code = $1 AND e.revoked_at IS NULL`, targetValue)
	case "major":
		rows, err = db.QueryContext(ctx,
			`SELECT DISTINCT e.student_id FROM entitlement e JOIN course c ON e.course_id = c.id WHERE c.major_id = $1 AND e.revoked_at IS NULL`, targetValue)
	default:
		return nil, ErrTargetInvalid
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListNotifications 分页拉取学员通知（unreadOnly=true 只看未读）。
func ListNotifications(ctx context.Context, db *sql.DB, studentID int64, unreadOnly bool, limit, offset int) ([]Notification, error) {
	if db == nil || studentID <= 0 {
		return nil, ErrNotificationNotFound
	}
	if limit <= 0 {
		limit = 20
	}
	q := `SELECT id, type, title, body, link, read_at, broadcast_id, created_at FROM notification WHERE student_id = $1`
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := db.QueryContext(ctx, q, studentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		var readAt, broadcastID, link sql.NullString
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &link, &readAt, &broadcastID, &n.CreatedAt); err != nil {
			return nil, err
		}
		if readAt.Valid {
			n.Read = true
		}
		if link.Valid {
			n.Link = link.String
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkRead 标记已读（只能标自己的通知）。
func MarkRead(ctx context.Context, db *sql.DB, notificationID, studentID int64) error {
	if db == nil || notificationID <= 0 || studentID <= 0 {
		return ErrNotificationNotFound
	}
	res, err := db.ExecContext(ctx,
		`UPDATE notification SET read_at = $1 WHERE id = $2 AND student_id = $3 AND read_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), notificationID, studentID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

// Notification 通知视图。
type Notification struct {
	ID        int64
	Type      string
	Title     string
	Body      string
	Link      string
	Read      bool
	CreatedAt string
}
