package course

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrContentRequestInput    = errors.New("下架申请参数非法")
	ErrContentRequestExists   = errors.New("该课程已有待处理的下架申请")
	ErrContentRequestNotFound = errors.New("下架申请不存在")
)

var contentReasonKinds = map[string]bool{
	"content_error":   true,
	"material_change": true,
	"personal":        true,
	"other":           true,
}

// ContentRequest 教师下架 / 紧急纠错申请。
type ContentRequest struct {
	ID          int64
	CourseCode  string
	CourseName  string
	TeacherID   int64
	TeacherName string
	Kind        string
	ReasonKind  string
	ReasonText  string
	Status      string
	HandledBy   *int64
	HandledAt   *string
	CreatedAt   string
}

func validContentKind(kind string) bool {
	return kind == "offline" || kind == "urgent_fix"
}

// CreateContentRequest 教师对自有已上架课程发起下架 / 紧急纠错申请。
func CreateContentRequest(ctx context.Context, db *sql.DB, in ContentRequest) (int64, error) {
	if db == nil || strings.TrimSpace(in.CourseCode) == "" || in.TeacherID <= 0 ||
		!validContentKind(in.Kind) || !contentReasonKinds[in.ReasonKind] {
		return 0, ErrContentRequestInput
	}
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM course WHERE course_code = $1 AND teacher_id = $2 AND deleted_at IS NULL`,
		in.CourseCode, in.TeacherID).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return 0, ErrCourseNotFound
		}
		return 0, err
	}
	if status != "published" {
		return 0, ErrInvalidTransition
	}
	var pending int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content_request WHERE course_code = $1 AND kind = $2 AND status = 'pending'`,
		in.CourseCode, in.Kind).Scan(&pending); err != nil {
		return 0, err
	}
	if pending > 0 {
		return 0, ErrContentRequestExists
	}
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO content_request(course_code, teacher_id, kind, reason_kind, reason_text, created_at)
		 VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		in.CourseCode, in.TeacherID, in.Kind, in.ReasonKind, strings.TrimSpace(in.ReasonText),
		time.Now().UTC().Format(time.RFC3339)).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "uq_content_request_pending") || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, ErrContentRequestExists
		}
		return 0, err
	}
	return id, nil
}

// ListContentRequests 管理员待办 / 教师自己的申请列表。
func ListContentRequests(ctx context.Context, db *sql.DB, status string, teacherID int64, limit, offset int) ([]ContentRequest, error) {
	if db == nil {
		return nil, ErrContentRequestInput
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	where := []string{"1=1"}
	args := make([]any, 0, 4)
	if status != "" {
		args = append(args, status)
		where = append(where, "r.status = $"+strconv.Itoa(len(args)))
	}
	if teacherID > 0 {
		args = append(args, teacherID)
		where = append(where, "r.teacher_id = $"+strconv.Itoa(len(args)))
	}
	args = append(args, limit, offset)
	q := `SELECT r.id, r.course_code, c.name, r.teacher_id, st.name, r.kind, r.reason_kind, r.reason_text,
	             r.status, r.handled_by, r.handled_at, r.created_at
	      FROM content_request r
	      JOIN course c ON c.course_code = r.course_code
	      JOIN staff st ON st.id = r.teacher_id
	      WHERE ` + strings.Join(where, " AND ") + `
	      ORDER BY CASE WHEN r.status = 'pending' THEN 0 ELSE 1 END, r.created_at DESC
	      LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ContentRequest, 0)
	for rows.Next() {
		var item ContentRequest
		var handledBy sql.NullInt64
		var handledAt sql.NullString
		if err := rows.Scan(&item.ID, &item.CourseCode, &item.CourseName, &item.TeacherID, &item.TeacherName,
			&item.Kind, &item.ReasonKind, &item.ReasonText, &item.Status, &handledBy, &handledAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if handledBy.Valid {
			item.HandledBy = &handledBy.Int64
		}
		if handledAt.Valid {
			item.HandledAt = &handledAt.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// HandleContentRequest 管理员批准/驳回；批准时下架课程（已购学员权益不受影响）。
func HandleContentRequest(ctx context.Context, db *sql.DB, id, adminID int64, approve bool) (ContentRequest, error) {
	if db == nil || id <= 0 || adminID <= 0 {
		return ContentRequest{}, ErrContentRequestInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ContentRequest{}, err
	}
	defer tx.Rollback()
	var req ContentRequest
	var handledBy sql.NullInt64
	var handledAt sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, course_code, teacher_id, kind, reason_kind, reason_text, status, handled_by, handled_at, created_at
		 FROM content_request WHERE id = $1`, id).
		Scan(&req.ID, &req.CourseCode, &req.TeacherID, &req.Kind, &req.ReasonKind, &req.ReasonText,
			&req.Status, &handledBy, &handledAt, &req.CreatedAt)
	if err == sql.ErrNoRows {
		return ContentRequest{}, ErrContentRequestNotFound
	}
	if err != nil {
		return ContentRequest{}, err
	}
	if req.Status != "pending" {
		return ContentRequest{}, ErrInvalidTransition
	}
	now := time.Now().UTC().Format(time.RFC3339)
	newStatus := "rejected"
	if approve {
		newStatus = "approved"
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE content_request SET status = $1, handled_by = $2, handled_at = $3 WHERE id = $4 AND status = 'pending'`,
		newStatus, adminID, now, id)
	if err != nil {
		return ContentRequest{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ContentRequest{}, ErrInvalidTransition
	}
	if approve {
		// 下架动作由管理员执行；已购学员权益与证书不变。
		if _, err := tx.ExecContext(ctx,
			`UPDATE course SET status = 'unpublished', updated_at = $1 WHERE course_code = $2 AND status = 'published'`,
			now, req.CourseCode); err != nil {
			return ContentRequest{}, err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE operation_slot SET status='removed' WHERE course_code=$1 AND status IN ('draft','active')`, req.CourseCode); err != nil {
			return ContentRequest{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ContentRequest{}, err
	}
	req.Status = newStatus
	req.HandledBy = &adminID
	req.HandledAt = &now
	return req, nil
}

// HasPendingUrgentFix 课程是否存在待处理的紧急纠错申请（暂停新测评/发证）。
func HasPendingUrgentFix(ctx context.Context, db *sql.DB, courseCode string) bool {
	if db == nil || courseCode == "" {
		return false
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content_request WHERE course_code = $1 AND kind = 'urgent_fix' AND status = 'pending'`,
		courseCode).Scan(&n); err != nil {
		return false
	}
	return n > 0
}
