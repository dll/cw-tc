package course

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrSlotInput    = errors.New("运营位参数非法")
	ErrSlotNotFound = errors.New("运营位不存在")
)

var slotPositions = map[string]bool{
	"home_banner":    true,
	"home_recommend": true,
	"plaza_top":      true,
	"topic":          true,
}

var slotStatuses = map[string]bool{
	"draft":   true,
	"active":  true,
	"expired": true,
	"removed": true,
}

// OperationSlot 站内运营位。
type OperationSlot struct {
	ID         int64
	Position   string
	CourseCode string
	Title      string
	ImageURL   string
	LinkURL    string
	StartsAt   string
	EndsAt     string
	Weight     int
	Status     string
	CreatedBy  *int64
}

func parseSlotTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}

func validateSlot(ctx context.Context, db *sql.DB, slot OperationSlot) error {
	if !slotPositions[slot.Position] || !slotStatuses[slot.Status] {
		return ErrSlotInput
	}
	start, err := parseSlotTime(slot.StartsAt)
	if err != nil {
		return ErrSlotInput
	}
	end, err := parseSlotTime(slot.EndsAt)
	if err != nil || !start.Before(end) {
		return ErrSlotInput
	}
	code := strings.TrimSpace(slot.CourseCode)
	if code == "" {
		if slot.Position == "home_recommend" || slot.Position == "plaza_top" {
			return ErrSlotInput
		}
		return nil
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if status != "published" {
		return ErrSlotInput
	}
	return nil
}

// CreateOperationSlot 管理员创建运营位。
func CreateOperationSlot(ctx context.Context, db *sql.DB, slot OperationSlot, adminID int64) (int64, error) {
	if db == nil || adminID <= 0 || strings.TrimSpace(slot.Title) == "" && strings.TrimSpace(slot.ImageURL) == "" {
		return 0, ErrSlotInput
	}
	if err := validateSlot(ctx, db, slot); err != nil {
		return 0, err
	}
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO operation_slot(position, course_code, title, image_url, link_url, starts_at, ends_at, weight, status, created_by)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		slot.Position, nullIfEmpty(slot.CourseCode), slot.Title, nullIfEmpty(slot.ImageURL),
		nullIfEmpty(slot.LinkURL), slot.StartsAt, slot.EndsAt, slot.Weight, slot.Status, adminID).Scan(&id)
	return id, err
}

// UpdateOperationSlot 管理员更新运营位（整条提交；status=removed 即下线）。
func UpdateOperationSlot(ctx context.Context, db *sql.DB, id int64, slot OperationSlot, adminID int64) error {
	if db == nil || id <= 0 || adminID <= 0 {
		return ErrSlotInput
	}
	if err := validateSlot(ctx, db, slot); err != nil {
		return err
	}
	res, err := db.ExecContext(ctx,
		`UPDATE operation_slot SET position=$1, course_code=$2, title=$3, image_url=$4, link_url=$5,
		        starts_at=$6, ends_at=$7, weight=$8, status=$9 WHERE id=$10`,
		slot.Position, nullIfEmpty(slot.CourseCode), slot.Title, nullIfEmpty(slot.ImageURL),
		nullIfEmpty(slot.LinkURL), slot.StartsAt, slot.EndsAt, slot.Weight, slot.Status, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrSlotNotFound
	}
	return nil
}

// RemoveOperationSlot 下线运营位（保留记录，不物理删除）。
func RemoveOperationSlot(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `UPDATE operation_slot SET status='removed' WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrSlotNotFound
	}
	return nil
}

// ExpireDueSlots 把已过结束时间的 active 运营位标记为 expired。
func ExpireDueSlots(ctx context.Context, db *sql.DB, now time.Time) error {
	if db == nil {
		return ErrSlotInput
	}
	_, err := db.ExecContext(ctx,
		`UPDATE operation_slot SET status='expired' WHERE status='active' AND ends_at < $1`,
		now.UTC().Format(time.RFC3339))
	return err
}

// ListOperationSlots 管理员列表（含 draft/expired/removed）。
func ListOperationSlots(ctx context.Context, db *sql.DB, now time.Time) ([]OperationSlot, error) {
	if err := ExpireDueSlots(ctx, db, now); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, position, course_code, title, image_url, link_url, starts_at, ends_at, weight, status, created_by
		 FROM operation_slot ORDER BY position, weight DESC, starts_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSlots(rows)
}

// PublicOperationSlots 公开只读：当前生效且课程仍上架的运营位。
func PublicOperationSlots(ctx context.Context, db *sql.DB, position string, now time.Time) ([]OperationSlot, error) {
	if db == nil || !slotPositions[position] {
		return nil, ErrSlotInput
	}
	if err := ExpireDueSlots(ctx, db, now); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT s.id, s.position, s.course_code, s.title, s.image_url, s.link_url, s.starts_at, s.ends_at, s.weight, s.status, s.created_by
		 FROM operation_slot s LEFT JOIN course c ON c.course_code = s.course_code
		 WHERE s.status='active' AND s.position=$1 AND s.starts_at <= $2 AND s.ends_at > $2
		   AND (s.course_code IS NULL OR c.status='published')
		 ORDER BY s.weight DESC, s.starts_at DESC, s.id DESC`, position, now.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSlots(rows)
}

func scanSlots(rows *sql.Rows) ([]OperationSlot, error) {
	items := make([]OperationSlot, 0)
	for rows.Next() {
		var item OperationSlot
		var code, image, link sql.NullString
		var createdBy sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Position, &code, &item.Title, &image, &link,
			&item.StartsAt, &item.EndsAt, &item.Weight, &item.Status, &createdBy); err != nil {
			return nil, err
		}
		item.CourseCode = code.String
		item.ImageURL = image.String
		item.LinkURL = link.String
		if createdBy.Valid {
			item.CreatedBy = &createdBy.Int64
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
