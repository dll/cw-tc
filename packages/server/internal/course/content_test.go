package course

import (
	"context"
	"database/sql"
	"testing"
)

// contentDB 含一门 platform 课 + 一门 upstream 课 + 教师。
func contentDB(t *testing.T) *sql.DB {
	t.Helper()
	database := courseDB(t)
	// platform 课（教师 id=1 本人）
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'TC-1000','platform','平台课','main',1,'building',1)`); err != nil {
		t.Fatal(err)
	}
	// upstream 课（教师不可自管）
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(2,'07-CN','upstream','上游课','main',1,'published',1)`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestAddLecturePlatformOnly(t *testing.T) {
	database := contentDB(t)
	defer database.Close()
	ctx := context.Background()

	// 平台课加讲次
	id, err := AddLecture(ctx, database, LectureInput{CourseID: 1, TeacherID: 1, Title: "第一讲"})
	if err != nil || id <= 0 {
		t.Fatalf("平台课加讲次失败: id=%d err=%v", id, err)
	}
	// 讲次 no 自增
	id2, _ := AddLecture(ctx, database, LectureInput{CourseID: 1, TeacherID: 1, Title: "第二讲"})
	var no2 int
	_ = database.QueryRow(`SELECT no FROM lecture WHERE id=?`, id2).Scan(&no2)
	if no2 != 2 {
		t.Fatalf("讲次 no 应自增为 2，实际 %d", no2)
	}
	// upstream 课加讲次 → 拒
	if _, err := AddLecture(ctx, database, LectureInput{CourseID: 2, TeacherID: 1, Title: "x"}); err != ErrNotPlatformCourse {
		t.Fatalf("上游课加讲次应报 ErrNotPlatformCourse，实际 %v", err)
	}
	// 非本人 → 拒
	if _, err := AddLecture(ctx, database, LectureInput{CourseID: 1, TeacherID: 99, Title: "x"}); err != ErrNotOwner {
		t.Fatalf("非本人加讲次应报 ErrNotOwner，实际 %v", err)
	}
}

func TestAddQuestionUniqueQNo(t *testing.T) {
	database := contentDB(t)
	defer database.Close()
	ctx := context.Background()

	in := QuestionInput{CourseID: 1, TeacherID: 1, QNo: "Q1", Level: "L1", Type: "single_choice", Gradability: "auto", Stem: "题干", Score: 5}
	if _, err := AddQuestion(ctx, database, in); err != nil {
		t.Fatal(err)
	}
	// qno 重复 → 拒
	if _, err := AddQuestion(ctx, database, in); err != ErrCourseInput {
		t.Fatalf("qno 重复应报 ErrCourseInput，实际 %v", err)
	}
	// 非法档位 → 拒
	if _, err := AddQuestion(ctx, database, QuestionInput{CourseID: 1, TeacherID: 1, QNo: "Q2", Level: "L9", Gradability: "auto", Stem: "x", Score: 1}); err != ErrCourseInput {
		t.Fatalf("非法档位应报 ErrCourseInput，实际 %v", err)
	}
}
