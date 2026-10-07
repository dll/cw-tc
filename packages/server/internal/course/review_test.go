package course

import (
	"context"
	"database/sql"
	"testing"
)

// reviewDB 含一名审核员 + 一门课程（course_code=C1），供审核测试用。
func reviewDB(t *testing.T) *sql.DB {
	t.Helper()
	database := courseDB(t)
	if _, err := database.Exec(`INSERT INTO course(id, course_code, source, name, line, major_id, status, teacher_id) VALUES(1,'C1','platform','课程','main',1,'submitted',1)`); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestSubmitReviewSixDimensions(t *testing.T) {
	database := reviewDB(t)
	defer database.Close()
	ctx := context.Background()

	// 满分通过：25 + 15×5 = 100
	ok, err := SubmitReview(ctx, database, ReviewInput{
		CourseCode: "C1", ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops",
		ScoreMaterial: 25, ScoreObjective: 15, ScoreCase: 15, ScoreLessonPlan: 15, ScoreSlides: 15, ScoreLab: 15,
		Verdict: "approved",
	})
	if err != nil || !ok {
		t.Fatalf("满分应通过: ok=%v err=%v", ok, err)
	}
	// round 递增（追加不覆盖）
	ok2, err := SubmitReview(ctx, database, ReviewInput{
		CourseCode: "C1", ContentVersion: 2, ReviewerID: 1, ReviewerRole: "admin_super",
		ScoreMaterial: 20, ScoreObjective: 13, ScoreCase: 13, ScoreLessonPlan: 13, ScoreSlides: 13, ScoreLab: 13,
		Verdict: "approved",
	})
	if err != nil || !ok2 {
		t.Fatalf("85 分应通过: ok=%v err=%v", ok2, err)
	}
	list, _ := ListReviews(ctx, database, "C1")
	if len(list) != 2 || list[0].Round != 2 || list[1].Round != 1 {
		t.Fatalf("历史应两条且 round 递增: %+v", list)
	}
	if list[1].TotalScore != 100 || list[0].TotalScore != 85 {
		t.Fatalf("total_score 派生值错误: %+v", list)
	}
}

func TestSubmitReviewRejectNeedsIssues(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	_, err := SubmitReview(ctx, database, ReviewInput{
		CourseCode: "C1", ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops",
		ScoreMaterial: 10, ScoreObjective: 8, ScoreCase: 8, ScoreLessonPlan: 8, ScoreSlides: 8, ScoreLab: 8,
		Verdict: "rejected",
	})
	if err != ErrRejectNeedsIssues {
		t.Fatalf("驳回无问题清单应报 ErrRejectNeedsIssues，实际 %v", err)
	}
}

func TestSubmitReviewPassLineSingleDimFloor(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	// 总分 89 ≥ 85，但「材料」14 < 25×0.6=15 → 单维下限不达标，应拒
	_, err := SubmitReview(ctx, database, ReviewInput{
		CourseCode: "C1", ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops",
		ScoreMaterial: 14, ScoreObjective: 15, ScoreCase: 15, ScoreLessonPlan: 15, ScoreSlides: 15, ScoreLab: 15,
		Verdict: "approved",
	})
	if err != ErrPassLineNotMet {
		t.Fatalf("单维低于下限应报 ErrPassLineNotMet，实际 %v", err)
	}
	// 六维范围超界
	if _, err := SubmitReview(ctx, database, ReviewInput{
		CourseCode: "C1", ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops",
		ScoreMaterial: 99, ScoreObjective: 15, ScoreCase: 15, ScoreLessonPlan: 15, ScoreSlides: 15, ScoreLab: 15,
		Verdict: "approved",
	}); err != ErrReviewInput {
		t.Fatalf("超界应报 ErrReviewInput，实际 %v", err)
	}
}
