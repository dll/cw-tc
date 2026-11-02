// Package offline 管理 App 离线下载授权（PRD §7.4 B-11 / 数据字典 §5.1.2）。
//
// 业务边界：
//   - 只有「已解锁」的学员能申请授权（免费课或有效权益）；
//   - 授权粒度到讲次，带离线有效期（默认 30 天，存 system_config.offline.valid_days）；
//   - 普通退款（normal）在同一事务内把该课授权置为 revoked；
//   - 本包只发授权，不搬文件 —— 文件由客户端加密存储，服务端只认权益与有效期。
package offline

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
)

var (
	ErrInput           = errors.New("离线授权参数非法")
	ErrCourseNotFound  = errors.New("课程不存在")
	ErrLectureNotFound = errors.New("讲次不存在")
	ErrForbidden       = errors.New("购买 / 解锁后才能下载")
	ErrNotFound        = errors.New("下载记录不存在")
)

type Grant struct {
	ID            int64   `json:"id"`
	CourseCode    string  `json:"course_code"`
	CourseName    string  `json:"course_name"`
	LectureNo     int     `json:"lecture_no"`
	LectureTitle  string  `json:"lecture_title"`
	GrantedAt     string  `json:"granted_at"`
	ExpiresAt     string  `json:"expires_at"`
	Status        string  `json:"status"`
	RevokedReason *string `json:"revoked_reason"`
	RemainingDays int     `json:"remaining_days"`
	// SizeBytes 由客户端统计本地占用空间；服务端不臆造数字。
	SizeBytes *int64 `json:"size_bytes"`
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func New(db *sql.DB) *Service {
	return &Service{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// NewWithNow 供测试注入时钟。
func NewWithNow(db *sql.DB, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, now: now}
}

// Request 申请 / 续期一讲的离线下载授权。
func (s *Service) Request(ctx context.Context, studentID int64, courseCode string, lectureNo, validDays int) (Grant, error) {
	if s == nil || s.db == nil || studentID <= 0 || courseCode == "" || lectureNo < 1 || validDays < 1 {
		return Grant{}, ErrInput
	}
	var courseID, priceCents int64
	var status string
	if err := s.db.QueryRowContext(ctx,
		`SELECT id, price_cents, status FROM course WHERE course_code = $1 AND deleted_at IS NULL`,
		courseCode).Scan(&courseID, &priceCents, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Grant{}, ErrCourseNotFound
		}
		return Grant{}, err
	}
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lecture WHERE course_id = $1 AND no = $2`, courseID, lectureNo).
		Scan(&exists); err != nil {
		return Grant{}, err
	}
	if exists == 0 {
		return Grant{}, ErrLectureNotFound
	}
	if !s.unlocked(ctx, studentID, courseID, priceCents, status) {
		return Grant{}, ErrForbidden
	}

	now := s.now().UTC()
	expires := now.Add(time.Duration(validDays) * 24 * time.Hour)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO download_grant(student_id, course_id, lecture_no, granted_at, expires_at, status)
		 VALUES($1,$2,$3,$4,$5,'valid')
		 ON CONFLICT(student_id, course_id, lecture_no)
		 DO UPDATE SET granted_at = excluded.granted_at, expires_at = excluded.expires_at,
		               status = 'valid', revoked_reason = NULL`,
		studentID, courseID, lectureNo, now.Format(time.RFC3339), expires.Format(time.RFC3339)); err != nil {
		return Grant{}, err
	}
	return s.get(ctx, studentID, courseID, lectureNo)
}

// List 返回学员的下载清单（只含自己在有效期 / 已过期 / 已撤销的记录）。
func (s *Service) List(ctx context.Context, studentID int64) ([]Grant, error) {
	if s == nil || s.db == nil || studentID <= 0 {
		return nil, ErrInput
	}
	now := s.now().UTC()
	if _, err := s.db.ExecContext(ctx,
		`UPDATE download_grant SET status = 'expired'
		 WHERE student_id = $1 AND status = 'valid' AND expires_at <= $2`,
		studentID, now.Format(time.RFC3339)); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT d.id, co.course_code, co.name, d.lecture_no, COALESCE(l.title,''),
		        d.granted_at, d.expires_at, d.status, d.revoked_reason
		 FROM download_grant d
		 JOIN course co ON co.id = d.course_id
		 LEFT JOIN lecture l ON l.course_id = d.course_id AND l.no = d.lecture_no
		 WHERE d.student_id = $1
		 ORDER BY d.expires_at DESC, d.id DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Grant, 0)
	for rows.Next() {
		item, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		item.RemainingDays = remainingDays(item.ExpiresAt, now)
		items = append(items, item)
	}
	return items, rows.Err()
}

// Delete 删除单讲下载授权（客户端同步清理本地文件）。
func (s *Service) Delete(ctx context.Context, studentID int64, courseCode string, lectureNo int) error {
	if s == nil || s.db == nil || studentID <= 0 || courseCode == "" || lectureNo < 1 {
		return ErrInput
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM download_grant
		 WHERE student_id = $1 AND lecture_no = $2
		   AND course_id = (SELECT id FROM course WHERE course_code = $3)`,
		studentID, lectureNo, courseCode)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByLectureNo 按讲次号删除下载授权（契约路径只带 lectureNo 时使用）。
func (s *Service) DeleteByLectureNo(ctx context.Context, studentID int64, lectureNo int) error {
	if s == nil || s.db == nil || studentID <= 0 || lectureNo < 1 {
		return ErrInput
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM download_grant WHERE student_id = $1 AND lecture_no = $2`, studentID, lectureNo)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeForCourse 普通退款时撤销该课全部有效下载授权（必须在退款事务内调用）。
func RevokeForCourse(ctx context.Context, tx execer, studentID, courseID int64, now time.Time) error {
	if tx == nil || studentID <= 0 || courseID <= 0 {
		return ErrInput
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE download_grant SET status = 'revoked', revoked_reason = 'order_refunded'
		 WHERE student_id = $1 AND course_id = $2 AND status = 'valid'`,
		studentID, courseID)
	return err
}

func (s *Service) get(ctx context.Context, studentID, courseID int64, lectureNo int) (Grant, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT d.id, co.course_code, co.name, d.lecture_no, COALESCE(l.title,''),
		        d.granted_at, d.expires_at, d.status, d.revoked_reason
		 FROM download_grant d
		 JOIN course co ON co.id = d.course_id
		 LEFT JOIN lecture l ON l.course_id = d.course_id AND l.no = d.lecture_no
		 WHERE d.student_id = $1 AND d.course_id = $2 AND d.lecture_no = $3`,
		studentID, courseID, lectureNo)
	item, err := scanGrant(row)
	if err != nil {
		return Grant{}, err
	}
	item.RemainingDays = remainingDays(item.ExpiresAt, s.now().UTC())
	return item, nil
}

func (s *Service) unlocked(ctx context.Context, studentID, courseID, priceCents int64, status string) bool {
	if priceCents == 0 && status == "published" {
		return true
	}
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE student_id = $1 AND course_id = $2 AND revoked_at IS NULL`,
		studentID, courseID).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

func scanGrant(scanner interface{ Scan(...any) error }) (Grant, error) {
	var item Grant
	var revoked sql.NullString
	if err := scanner.Scan(&item.ID, &item.CourseCode, &item.CourseName, &item.LectureNo,
		&item.LectureTitle, &item.GrantedAt, &item.ExpiresAt, &item.Status, &revoked); err != nil {
		return Grant{}, err
	}
	if revoked.Valid {
		reason := revoked.String
		item.RevokedReason = &reason
	}
	return item, nil
}

func remainingDays(expiresAt string, now time.Time) int {
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return 0
	}
	if !expires.After(now) {
		return 0
	}
	return int(math.Ceil(expires.Sub(now).Hours() / 24))
}
