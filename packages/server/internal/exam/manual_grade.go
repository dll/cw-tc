package exam

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotYourCourse = errors.New("无权评阅该课程")

// ManualReviewItem 是待阅卷 / 已阅卷的主观题条目（教师工作台）。
type ManualReviewItem struct {
	ResultID      int64
	ExamSessionID int64
	StudentID     int64
	CourseID      int64
	CourseName    string
	Level         string
	AutoScore     float64
	ManualScore   float64
	TotalScore    float64
}

// ListManualReviews 列出**该教师名下课程**的、含主观题的测评结果（强制归属条件，铁律 8）。
func ListManualReviews(ctx context.Context, db *sql.DB, teacherID int64) ([]ManualReviewItem, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT r.id, r.exam_session_id, r.student_id, r.course_id, c.name, r.level, r.auto_score, r.manual_score, r.total_score
		 FROM exam_result r JOIN course c ON r.course_id = c.id
		 WHERE c.teacher_id = $1 AND r.detail LIKE '%"manual":true%'
		 ORDER BY r.id DESC`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ManualReviewItem, 0)
	for rows.Next() {
		var it ManualReviewItem
		if err := rows.Scan(&it.ResultID, &it.ExamSessionID, &it.StudentID, &it.CourseID, &it.CourseName,
			&it.Level, &it.AutoScore, &it.ManualScore, &it.TotalScore); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GradeManual 评阅一道主观题：写 manual_review（按评阅人 upsert），并重算该结果的总分 / 是否通过。
// 归属校验在 API 层完成（本函数只做数据写入与重算）。
func GradeManual(ctx context.Context, db *sql.DB, resultID, questionID, reviewerID int64, score float64, comment, points string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// upsert：同一评阅人对同一题只保留一条（重评覆盖）。
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO manual_review(exam_result_id, question_id, reviewer_id, points, score, comment)
		 VALUES($1,$2,$3,$4,$5,$6)
		 ON CONFLICT(exam_result_id, question_id, reviewer_id) DO UPDATE SET points=excluded.points, score=excluded.score, comment=excluded.comment`,
		resultID, questionID, reviewerID, points, score, comment); err != nil {
		return err
	}

	// 重算该结果的主观总分（取该题所有评阅人里最新一条，简单起见取 SUM；单人评阅时即自身）。
	var manualScore float64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(score), 0) FROM manual_review WHERE exam_result_id = $1`, resultID).Scan(&manualScore); err != nil {
		return err
	}

	var courseID int64
	var autoScore float64
	if err := tx.QueryRowContext(ctx,
		`SELECT course_id, auto_score FROM exam_result WHERE id = $1`, resultID).Scan(&courseID, &autoScore); err != nil {
		return err
	}
	total := autoScore + manualScore
	passLine := passLineFor(ctx, tx, courseID, levelOf(ctx, tx, resultID))
	isPass := total >= passLine
	band := "fail"
	if isPass {
		band = "pass"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`UPDATE exam_result SET manual_score = $1, total_score = $2, band = $3, is_pass = $4, graded_at = $5 WHERE id = $6`,
		manualScore, total, band, isPass, now, resultID); err != nil {
		return err
	}
	return tx.Commit()
}

// levelOf 读取结果档位（供重算用，失败回退空串）。
func levelOf(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, resultID int64) string {
	var lv string
	_ = q.QueryRowContext(ctx, `SELECT level FROM exam_result WHERE id = $1`, resultID).Scan(&lv)
	return lv
}
