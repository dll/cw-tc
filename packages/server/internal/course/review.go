package course

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrReviewInput       = errors.New("审核参数非法")
	ErrRejectNeedsIssues = errors.New("驳回必须填写问题清单")
	ErrPassLineNotMet    = errors.New("通过线未达标：总分或单维低于下限")
	ErrRoleInvalid       = errors.New("审核角色非法")
)

// 六维满分（权重合计 = 100，铁律 1 §B-9：材料 25 + 其余五维各 15）。
const (
	fullMaterial  = 25.0
	fullObjective = 15.0
	fullCase      = 15.0
	fullLesson    = 15.0
	fullSlides    = 15.0
	fullLab       = 15.0
)

// 通过线（§B-9）：总分 ≥ 85 且单维 ≥ 满分 × 60%（单维下限是硬条件）。
const (
	passTotal   = 85.0
	singleRatio = 0.6
)

// ReviewInput 六维审核入参。
type ReviewInput struct {
	CourseCode      string
	ContentVersion  int
	ReviewerID      int64
	ReviewerRole    string
	ScoreMaterial   float64
	ScoreObjective  float64
	ScoreCase       float64
	ScoreLessonPlan float64
	ScoreSlides     float64
	ScoreLab        float64
	DimensionNotes  string
	IssueList       string
	Verdict         string
}

// SubmitReview 提交审核：六维打分，驳回必填 issue_list，round 递增追加（不覆盖历史）。
func SubmitReview(ctx context.Context, db *sql.DB, in ReviewInput) (bool, error) {
	if db == nil || in.CourseCode == "" || in.ReviewerID <= 0 || in.ContentVersion < 1 {
		return false, ErrReviewInput
	}
	if in.ReviewerRole != "admin_ops" && in.ReviewerRole != "admin_super" {
		return false, ErrRoleInvalid
	}
	if in.Verdict != "approved" && in.Verdict != "rejected" {
		return false, ErrReviewInput
	}
	// 六维范围校验
	for _, s := range []struct{ v, full float64 }{
		{in.ScoreMaterial, fullMaterial}, {in.ScoreObjective, fullObjective},
		{in.ScoreCase, fullCase}, {in.ScoreLessonPlan, fullLesson},
		{in.ScoreSlides, fullSlides}, {in.ScoreLab, fullLab},
	} {
		if s.v < 0 || s.v > s.full {
			return false, ErrReviewInput
		}
	}
	total := in.ScoreMaterial + in.ScoreObjective + in.ScoreCase + in.ScoreLessonPlan + in.ScoreSlides + in.ScoreLab
	// 驳回必须带问题清单
	if in.Verdict == "rejected" && (in.IssueList == "" || in.IssueList == "[]" || in.IssueList == "null") {
		return false, ErrRejectNeedsIssues
	}
	// 通过线（单维下限硬条件）：approved 必须总分与单维都达标
	if in.Verdict == "approved" {
		if total < passTotal || in.ScoreMaterial < fullMaterial*singleRatio ||
			in.ScoreObjective < fullObjective*singleRatio || in.ScoreCase < fullCase*singleRatio ||
			in.ScoreLessonPlan < fullLesson*singleRatio || in.ScoreSlides < fullSlides*singleRatio ||
			in.ScoreLab < fullLab*singleRatio {
			return false, ErrPassLineNotMet
		}
	}
	// round = 现有最大 + 1（追加，不覆盖）
	var round int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(round), 0) FROM course_review WHERE course_code = $1`, in.CourseCode).Scan(&round); err != nil {
		return false, err
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO course_review(course_code, round, content_version, reviewer_id, reviewer_role,
			score_material, score_objective, score_case, score_lesson_plan, score_slides, score_lab,
			dimension_notes, issue_list, verdict, reviewed_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		in.CourseCode, round+1, in.ContentVersion, in.ReviewerID, in.ReviewerRole,
		in.ScoreMaterial, in.ScoreObjective, in.ScoreCase, in.ScoreLessonPlan, in.ScoreSlides, in.ScoreLab,
		in.DimensionNotes, in.IssueList, in.Verdict, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	return total >= passTotal, nil
}

// Review 审核历史条目。
type Review struct {
	ID             int64
	Round          int
	ContentVersion int
	ReviewerID     int64
	ReviewerRole   string
	TotalScore     float64
	Verdict        string
	ReviewedAt     string
}

// ListReviews 按课程拉审核历史（新→旧，可对比「62 → 88 分」）。
func ListReviews(ctx context.Context, db *sql.DB, courseCode string) ([]Review, error) {
	if db == nil || courseCode == "" {
		return nil, ErrReviewInput
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, round, content_version, reviewer_id, reviewer_role, total_score, verdict, reviewed_at
		 FROM course_review WHERE course_code = $1 ORDER BY round DESC`, courseCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Review, 0)
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.Round, &r.ContentVersion, &r.ReviewerID, &r.ReviewerRole,
			&r.TotalScore, &r.Verdict, &r.ReviewedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
