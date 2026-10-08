package exam

import (
	"context"
	"testing"
)

func TestListManualReviewsAndGrade(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`UPDATE course SET cert_pass_lines = '{"L1":4.5}' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	grantEntitlement(t, database, 1, 1)
	paper, err := BuildPaper(ctx, database, 1, "L1", 0)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}
	// 客观题全对 3 分，主观题 1.5 分留待阅卷
	if _, err := SubmitExam(ctx, database, sid, map[string]string{
		"L1-01-01": "A", "L1-01-02": "对", "L1-02-01": "指针用于间接访问内存",
	}); err != nil {
		t.Fatal(err)
	}

	// 待阅卷清单（教师 1）
	items, err := ListManualReviews(ctx, database, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("待阅卷应 1 条，实际 %d", len(items))
	}
	// 其他教师不可见（归属条件）
	others, err := ListManualReviews(ctx, database, 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(others) != 0 {
		t.Fatalf("他人课程应 0 条，实际 %d", len(others))
	}

	var resultID, qid int64
	_ = database.QueryRow(`SELECT id FROM exam_result WHERE exam_session_id = $1`, sid).Scan(&resultID)
	_ = database.QueryRow(`SELECT id FROM question WHERE qno = 'L1-02-01'`).Scan(&qid)

	// 评阅主观题：满分 1.5，总分应 4.5，通过（>= 4.5）
	if err := GradeManual(ctx, database, resultID, qid, 1, 1.5, "要点齐全", `[{"point":"间接访问"}]`); err != nil {
		t.Fatal(err)
	}
	var manual, total float64
	var isPass bool
	_ = database.QueryRow(`SELECT manual_score, total_score, is_pass FROM exam_result WHERE id = $1`, resultID).Scan(&manual, &total, &isPass)
	if manual != 1.5 || total != 4.5 {
		t.Fatalf("主观分/总分错误: manual=%v total=%v", manual, total)
	}
	if !isPass {
		t.Fatalf("应通过")
	}
	// 重评覆盖（upsert 不重复累计）
	if err := GradeManual(ctx, database, resultID, qid, 1, 0.5, "扣分", `[]`); err != nil {
		t.Fatal(err)
	}
	_ = database.QueryRow(`SELECT manual_score FROM exam_result WHERE id = $1`, resultID).Scan(&manual)
	if manual != 0.5 {
		t.Fatalf("重评应覆盖为 0.5，实际 %v", manual)
	}
}
