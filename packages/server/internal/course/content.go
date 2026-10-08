package course

import (
	"context"
	"database/sql"
	"errors"
)

var ErrNotPlatformCourse = errors.New("非平台课程，不可教师自管")

// LectureInput 添加讲次入参。
type LectureInput struct {
	CourseID  int64
	TeacherID int64
	Title     string
	IsPreview bool
}

// AddLecture 教师给 platform 课添加讲次（origin=platform，no 自增，铁律 1 平台自管）。
func AddLecture(ctx context.Context, db *sql.DB, in LectureInput) (int64, error) {
	if in.Title == "" {
		return 0, ErrCourseInput
	}
	if err := checkPlatformOwnership(ctx, db, in.CourseID, in.TeacherID); err != nil {
		return 0, err
	}
	var no int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(no), 0) FROM lecture WHERE course_id = $1`, in.CourseID).Scan(&no); err != nil {
		return 0, err
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO lecture(course_id, no, title, is_preview, origin) VALUES($1,$2,$3,$4,'platform')`,
		in.CourseID, no+1, in.Title, in.IsPreview)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// QuestionInput 添加题目入参。
type QuestionInput struct {
	CourseID    int64
	TeacherID   int64
	QNo         string
	Level       string
	Type        string
	Gradability string
	Stem        string
	Options     string
	Answer      string
	Analysis    string
	Score       float64
}

// AddQuestion 教师给 platform 课添加题目（qno 课程内唯一）。
func AddQuestion(ctx context.Context, db *sql.DB, in QuestionInput) (int64, error) {
	if in.QNo == "" || in.Stem == "" || in.Score < 0 ||
		(in.Level != "L1" && in.Level != "L2" && in.Level != "L3") ||
		(in.Gradability != "auto" && in.Gradability != "semi_auto" && in.Gradability != "manual") {
		return 0, ErrCourseInput
	}
	if err := checkPlatformOwnership(ctx, db, in.CourseID, in.TeacherID); err != nil {
		return 0, err
	}
	var dup int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM question WHERE course_id = $1 AND qno = $2`, in.CourseID, in.QNo).Scan(&dup); err != nil {
		return 0, err
	}
	if dup > 0 {
		return 0, ErrCourseInput // qno 重复
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO question(course_id, qno, level, type, gradability, stem, options, answer, analysis, score)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		in.CourseID, in.QNo, in.Level, in.Type, in.Gradability, in.Stem, in.Options, in.Answer, in.Analysis, in.Score)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// checkPlatformOwnership 校验课程是 platform 且为教师本人（铁律 8 归属强制）。
func checkPlatformOwnership(ctx context.Context, db *sql.DB, courseID, teacherID int64) error {
	if db == nil || courseID <= 0 || teacherID <= 0 {
		return ErrCourseInput
	}
	var source string
	var teacher sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT source, teacher_id FROM course WHERE id = $1`, courseID).Scan(&source, &teacher); err != nil {
		if err == sql.ErrNoRows {
			return ErrCourseNotFound
		}
		return err
	}
	if source != "platform" {
		return ErrNotPlatformCourse
	}
	if !teacher.Valid || teacher.Int64 != teacherID {
		return ErrNotOwner
	}
	return nil
}
