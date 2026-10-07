package course

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrCertConfigRequired = errors.New("platform 课上架前必填证书名与合格线")

// ReviewCourse 管理员审核：submitted → approved/rejected，六维打分落 course_review + 更新课程状态。
func ReviewCourse(ctx context.Context, db *sql.DB, in ReviewInput) error {
	if db == nil || in.CourseCode == "" || in.ReviewerID <= 0 {
		return ErrCourseInput
	}
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM course WHERE course_code = ?`, in.CourseCode).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if status != "submitted" {
		return ErrInvalidTransition
	}
	if _, err := SubmitReview(ctx, db, in); err != nil {
		return err
	}
	rejectReason := ""
	if in.Verdict == "rejected" {
		rejectReason = in.IssueList
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`UPDATE course SET status=?, reviewed_at=?, reviewed_by=?, reject_reason=? WHERE course_code=?`,
		in.Verdict, now, in.ReviewerID, rejectReason, in.CourseCode)
	return err
}

// PublishCourse 管理员上架：approved → published（platform 课上架前 cert_names/cert_pass_lines 必填）。
func PublishCourse(ctx context.Context, db *sql.DB, courseCode string, adminID int64) error {
	if db == nil || courseCode == "" || adminID <= 0 {
		return ErrCourseInput
	}
	var status, source string
	var certNames, certLines sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT status, source, cert_names, cert_pass_lines FROM course WHERE course_code = ?`, courseCode).
		Scan(&status, &source, &certNames, &certLines); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if status != "approved" {
		return ErrInvalidTransition
	}
	// platform 课上架前必填（上游课已有冻结值）
	if source == "platform" && (!certNames.Valid || certNames.String == "" || !certLines.Valid || certLines.String == "") {
		return ErrCertConfigRequired
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`UPDATE course SET status='published', updated_at=? WHERE course_code=?`, now, courseCode)
	return err
}
