package exam

import (
	"context"
	"testing"
)

func TestSubmitExamAutoGradeAndPass(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`UPDATE course SET cert_pass_lines = '{"L1":1.5}' WHERE id = 1`); err != nil {
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
	// 答对 2 道客观题（各 1.5 分），主观题不判。
	res, err := SubmitExam(ctx, database, sid, map[string]string{
		"L1-01-01": "A", "L1-01-02": "对", "L1-02-01": "任意",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.AutoScore != 3.0 {
		t.Fatalf("客观得分应为 3.0，实际 %v", res.AutoScore)
	}
	if !res.IsPass || res.Band != "pass" {
		t.Fatalf("应通过: is_pass=%v band=%s", res.IsPass, res.Band)
	}
	// 会话应已转 graded，且答案落库
	var status string
	_ = database.QueryRow(`SELECT status FROM exam_session WHERE id = ?`, sid).Scan(&status)
	if status != "graded" {
		t.Fatalf("会话状态应 graded，实际 %s", status)
	}
	var n int
	_ = database.QueryRow(`SELECT COUNT(*) FROM exam_result WHERE exam_session_id = ?`, sid).Scan(&n)
	if n != 1 {
		t.Fatalf("应写 1 条成绩，实际 %d", n)
	}
}

func TestSubmitExamWrongAnswerFails(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`UPDATE course SET cert_pass_lines = '{"L1":10}' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, _ := StartSession(ctx, database, 1, 1, "L1", paper)
	// 全答错
	res, err := SubmitExam(ctx, database, sid, map[string]string{"L1-01-01": "B", "L1-01-02": "错"})
	if err != nil {
		t.Fatal(err)
	}
	if res.AutoScore != 0 {
		t.Fatalf("全错应得 0，实际 %v", res.AutoScore)
	}
	if res.IsPass || res.Band != "fail" {
		t.Fatalf("应不通过: is_pass=%v band=%s", res.IsPass, res.Band)
	}
}

func TestSubmitExamRejectsNonOngoing(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, _ := StartSession(ctx, database, 1, 1, "L1", paper)
	if _, err := SubmitExam(ctx, database, sid, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	// 已交卷，再次交卷应拒绝
	if _, err := SubmitExam(ctx, database, sid, map[string]string{}); err != ErrSessionNotOngoing {
		t.Fatalf("已交卷再交应报 ErrSessionNotOngoing，实际 %v", err)
	}
}
