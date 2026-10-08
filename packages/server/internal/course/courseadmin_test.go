package course

import (
	"context"
	"testing"
)

func TestCreatePlatformCourseAndLifecycle(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 建课：自动分配 TC- 码，source=platform，初始 proposed
	code, err := CreatePlatformCourse(ctx, database, CreateCourseInput{
		TeacherID: 1, Name: "平台新课", Line: "main", MajorID: 1, LectureCount: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	var source, status string
	if err := database.QueryRow(`SELECT source, status FROM course WHERE course_code=$1`, code).Scan(&source, &status); err != nil {
		t.Fatal(err)
	}
	if source != "platform" || status != "proposed" {
		t.Fatalf("建课应为 platform/proposed，实际 %s/%s", source, status)
	}

	// proposed → building
	if err := StartBuilding(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}
	// 送审（major 已填）→ submitted
	if err := SubmitCourse(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code=$1`, code).Scan(&status)
	if status != "submitted" {
		t.Fatalf("送审后应为 submitted，实际 %s", status)
	}
	// submitted → building 是非法跃迁
	if err := StartBuilding(ctx, database, code, 1); err != ErrInvalidTransition {
		t.Fatalf("submitted→building 应报 ErrInvalidTransition，实际 %v", err)
	}
}

func TestSubmitCourseRequiresMajorAndOwnership(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 无专业分类的课
	code, _ := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "无专业", Line: "main"})
	if err := SubmitCourse(ctx, database, code, 1); err != ErrMajorRequired {
		t.Fatalf("无专业送审应报 ErrMajorRequired，实际 %v", err)
	}
	// 非本人送审
	code2, _ := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "归属", Line: "main", MajorID: 1})
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(2,'t2','乙老师','teacher',X'01','admin')`); err != nil {
		t.Fatal(err)
	}
	if err := SubmitCourse(ctx, database, code2, 2); err != ErrNotOwner {
		t.Fatalf("非本人送审应报 ErrNotOwner，实际 %v", err)
	}
}

func TestCourseAdminEdgeCases(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 课程不存在
	if err := SubmitCourse(ctx, database, "TC-NOPE", 1); err != ErrCourseNotFound {
		t.Fatalf("不存在的课程应报 ErrCourseNotFound，实际 %v", err)
	}
	// 非法 line
	if _, err := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "x", Line: "bad"}); err != ErrCourseInput {
		t.Fatalf("非法 line 应报 ErrCourseInput，实际 %v", err)
	}
	// 送审幂等：submitted 再送审返回 nil
	code, _ := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "幂等", Line: "main", MajorID: 1})
	if err := SubmitCourse(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}
	if err := SubmitCourse(ctx, database, code, 1); err != nil {
		t.Fatalf("重复送审应幂等返回 nil，实际 %v", err)
	}
}

func TestSetPriceEdges(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 不存在的课程
	if err := SetPrice(ctx, database, "TC-NOPE", 100); err != ErrCourseNotFound {
		t.Fatalf("改价不存在课程应报 ErrCourseNotFound，实际 %v", err)
	}
	// 非法参数
	if err := SetPrice(ctx, database, "", -1); err != ErrCourseInput {
		t.Fatalf("非法改价参数应报 ErrCourseInput，实际 %v", err)
	}
}
