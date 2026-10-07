package course

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrCourseInput       = errors.New("建课参数非法")
	ErrCourseNotFound    = errors.New("课程不存在")
	ErrNotOwner          = errors.New("非本人课程，无权操作")
	ErrMajorRequired     = errors.New("上架前必填专业分类")
	ErrInvalidTransition = errors.New("非法状态跃迁")
)

// CreateCourseInput 教师建课入参。
type CreateCourseInput struct {
	TeacherID    int64
	Name         string
	Line         string
	MajorID      int64 // 可空，送审前必填
	Intro        string
	LectureCount int
}

// CreatePlatformCourse 教师建课（PRD §7.9 J-1）：source=platform，自动分配课程码，初始 proposed。
func CreatePlatformCourse(ctx context.Context, db *sql.DB, in CreateCourseInput) (string, error) {
	if db == nil || in.TeacherID <= 0 || in.Name == "" || (in.Line != "main" && in.Line != "aux") {
		return "", ErrCourseInput
	}
	code, err := AllocateCourseCode(ctx, db)
	if err != nil {
		return "", err
	}
	var majorArg any
	if in.MajorID > 0 {
		majorArg = in.MajorID
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO course(course_code, source, name, intro, line, lecture_count, major_id, teacher_id, status)
		 VALUES(?, 'platform', ?, ?, ?, ?, ?, ?, 'proposed')`,
		code, in.Name, in.Intro, in.Line, in.LectureCount, majorArg, in.TeacherID); err != nil {
		return "", err
	}
	return code, nil
}

// StartBuilding 立项 → 建设中（建课两态的第二态）。
func StartBuilding(ctx context.Context, db *sql.DB, courseCode string, teacherID int64) error {
	return transition(ctx, db, courseCode, teacherID, "building",
		map[string]bool{"proposed": true}, false)
}

// SubmitCourse 教师提交送审：proposed / building → submitted，校验归属 + 专业必填。
func SubmitCourse(ctx context.Context, db *sql.DB, courseCode string, teacherID int64) error {
	return transition(ctx, db, courseCode, teacherID, "submitted",
		map[string]bool{"proposed": true, "building": true}, true)
}

// transition 统一状态机跃迁：校验归属 + 前置状态合法性 + 送审前必填 major_id。
func transition(ctx context.Context, db *sql.DB, courseCode string, teacherID int64,
	to string, from map[string]bool, needMajor bool) error {
	if db == nil || courseCode == "" || teacherID <= 0 {
		return ErrCourseInput
	}
	var owner sql.NullInt64
	var status string
	var majorNull sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT teacher_id, status, major_id FROM course WHERE course_code = ?`, courseCode).
		Scan(&owner, &status, &majorNull); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if !owner.Valid || owner.Int64 != teacherID {
		return ErrNotOwner
	}
	if needMajor && !majorNull.Valid {
		return ErrMajorRequired
	}
	if status == to { // 幂等：已是目标状态
		return nil
	}
	if !from[status] {
		return ErrInvalidTransition
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if to == "submitted" {
		_, err := db.ExecContext(ctx,
			`UPDATE course SET status='submitted', submitted_at=?, updated_at=? WHERE course_code=?`,
			now, now, courseCode)
		return err
	}
	_, err := db.ExecContext(ctx,
		`UPDATE course SET status=?, updated_at=? WHERE course_code=?`,
		to, now, courseCode)
	return err
}
