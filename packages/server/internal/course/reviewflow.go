package course

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrCertConfigRequired = errors.New("platform 课上架前必填证书名与合格线")

// ReviewCourse 管理员审核：submitted → approved/rejected，六维打分落 course_review + 更新课程状态。
func ReviewCourse(ctx context.Context, db *sql.DB, in ReviewInput) error {
	if db == nil || in.CourseCode == "" || in.ReviewerID <= 0 {
		return ErrCourseInput
	}
	var status string
	var contentVersion int
	if err := db.QueryRowContext(ctx,
		`SELECT status, content_version FROM course WHERE course_code = $1`, in.CourseCode).Scan(&status, &contentVersion); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if status != "submitted" {
		return ErrInvalidTransition
	}
	// 审核绑定课程当前内容版本（不信任客户端传入的版本号）。
	in.ContentVersion = contentVersion
	if _, err := SubmitReview(ctx, db, in); err != nil {
		return err
	}
	rejectReason := ""
	if in.Verdict == "rejected" {
		rejectReason = in.IssueList
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`UPDATE course SET status=$1, reviewed_at=$2, reviewed_by=$3, reject_reason=$4 WHERE course_code=$5`,
		in.Verdict, now, in.ReviewerID, rejectReason, in.CourseCode)
	return err
}

// PublishCourse 管理员上架/重新上架：approved 或 unpublished → published。
// 上架前三项配置必须齐备：专业（本函数校验）、价格（默认值可改）、证书配置（platform 必填）。
func PublishCourse(ctx context.Context, db *sql.DB, courseCode string, adminID int64) error {
	if db == nil || courseCode == "" || adminID <= 0 {
		return ErrCourseInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status, source string
	var certNames, certLines sql.NullString
	var majorID sql.NullInt64
	var contentVersion int
	if err := tx.QueryRowContext(ctx,
		`SELECT status, source, cert_names, cert_pass_lines, major_id, content_version FROM course WHERE course_code = $1`, courseCode).
		Scan(&status, &source, &certNames, &certLines, &majorID, &contentVersion); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	// approved → published（首次上架）；unpublished → published（重新上架）
	if status != "approved" && status != "unpublished" {
		return ErrInvalidTransition
	}
	if !majorID.Valid {
		return ErrMajorRequired
	}
	// platform 课上架前必填（上游课已有冻结值）
	if source == "platform" && (!certNames.Valid || certNames.String == "" || !certLines.Valid || certLines.String == "") {
		return ErrCertConfigRequired
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`UPDATE course SET status='published', published_version=content_version, updated_at=$1 WHERE course_code=$2`,
		now, courseCode); err != nil {
		return err
	}
	// 每次发布追加改版记录（revision_no 递增，历史不可覆盖）。
	var revisionNo int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision_no),0)+1 FROM course_revision WHERE course_code = $1`, courseCode).
		Scan(&revisionNo); err != nil {
		return err
	}
	digest := fmt.Sprintf("从 %s 上架到 published（内容版本 v%d）", status, contentVersion)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO course_revision(course_code, revision_no, content_version, changed_digest, published_at) VALUES($1,$2,$3,$4,$5)`,
		courseCode, revisionNo, contentVersion, digest, now); err != nil {
		return err
	}
	return tx.Commit()
}

// UnpublishCourse 管理员下架：published → unpublished。
// 已购学员的权益与证书不受影响（仍可学可测），不在此处撤销。
func UnpublishCourse(ctx context.Context, db *sql.DB, courseCode string, adminID int64) error {
	if db == nil || courseCode == "" || adminID <= 0 {
		return ErrCourseInput
	}
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM course WHERE course_code = $1`, courseCode).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if status != "published" {
		return ErrInvalidTransition
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`UPDATE course SET status='unpublished', updated_at=$1 WHERE course_code=$2`, now, courseCode)
	return err
}
