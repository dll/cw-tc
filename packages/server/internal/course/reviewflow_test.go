package course

import (
	"context"
	"testing"
)

func TestReviewAndPublishFlow(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 建课 → 送审（带专业）
	code, _ := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "送审课", Line: "main", MajorID: 1})
	if err := SubmitCourse(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}

	// 审核通过（六维满分）
	review := ReviewInput{CourseCode: code, ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops", ScoreMaterial: 25, ScoreObjective: 15, ScoreCase: 15, ScoreLessonPlan: 15, ScoreSlides: 15, ScoreLab: 15, Verdict: "approved"}
	if err := ReviewCourse(ctx, database, review); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	var status string
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code=?`, code).Scan(&status)
	if status != "approved" {
		t.Fatalf("审核通过后应为 approved，实际 %s", status)
	}
	// 上架：platform 课缺 cert_names/cert_pass_lines → 拒
	if err := PublishCourse(ctx, database, code, 1); err != ErrCertConfigRequired {
		t.Fatalf("缺证书配置应报 ErrCertConfigRequired，实际 %v", err)
	}
	// 补齐证书配置后上架
	if _, err := database.Exec(`UPDATE course SET cert_names='{"L1":"初级"}', cert_pass_lines='{"L1":60}' WHERE course_code=?`, code); err != nil {
		t.Fatal(err)
	}
	if err := PublishCourse(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code=?`, code).Scan(&status)
	if status != "published" {
		t.Fatalf("上架后应为 published，实际 %s", status)
	}
}

func TestReviewRejectedSetsReason(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	code, _ := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "驳回课", Line: "main", MajorID: 1})
	if err := SubmitCourse(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}
	// 驳回（六维低分 + 问题清单）
	err := ReviewCourse(ctx, database, ReviewInput{
		CourseCode: code, ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops",
		ScoreMaterial: 10, ScoreObjective: 8, ScoreCase: 8, ScoreLessonPlan: 8, ScoreSlides: 8, ScoreLab: 8,
		IssueList: `[{"dim":"material","issue":"讲义缺失"}]`, Verdict: "rejected",
	})
	if err != nil {
		t.Fatal(err)
	}
	var status, reason string
	_ = database.QueryRow(`SELECT status, reject_reason FROM course WHERE course_code=?`, code).Scan(&status, &reason)
	if status != "rejected" || reason == "" {
		t.Fatalf("驳回后应 rejected 且带原因，实际 %s / %s", status, reason)
	}
}

func TestReviewFlowEdgeCases(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()

	// 审核不存在的课程
	if err := ReviewCourse(ctx, database, ReviewInput{CourseCode: "TC-NOPE", Verdict: "approved", ReviewerID: 1, ReviewerRole: "admin_ops", ScoreMaterial: 25, ScoreObjective: 15, ScoreCase: 15, ScoreLessonPlan: 15, ScoreSlides: 15, ScoreLab: 15}); err != ErrCourseNotFound {
		t.Fatalf("审核不存在课程应报 ErrCourseNotFound，实际 %v", err)
	}
	// 未审核通过（仍 submitted）直接上架 → 非法跃迁
	code, _ := CreatePlatformCourse(ctx, database, CreateCourseInput{TeacherID: 1, Name: "未审核", Line: "main", MajorID: 1})
	if err := SubmitCourse(ctx, database, code, 1); err != nil {
		t.Fatal(err)
	}
	if err := PublishCourse(ctx, database, code, 1); err != ErrInvalidTransition {
		t.Fatalf("未 approved 上架应报 ErrInvalidTransition，实际 %v", err)
	}
}
