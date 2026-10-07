package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// buildQuestionFixture 生成一门带题库的上游课程：2 讲 + 2 题（1 客观 + 1 主观）。
func buildQuestionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	courseDir := filepath.Join(root, "AI化教学创新03-CF")
	if err := os.MkdirAll(filepath.Join(courseDir, "测试题集"), 0o755); err != nil {
		t.Fatal(err)
	}
	bank := QuestionBank{
		Questions: []Question{
			{ID: "L1-01-01", Level: "L1", Lecture: "01", Type: "单选", Score: 1.5, Stem: "赋值是表达式吗？",
				Options: []string{"- A. 是", "- B. 否"}, Answer: "A", Explanation: "赋值有值", AutoGradable: true},
			{ID: "L2-02-01", Level: "L2", Lecture: "02", Type: "简答", Score: 5, Stem: "简述指针作用。",
				AnswerKey: "间接访问内存", Scoring: "答出间接访问给 5 分", AutoGradable: false},
		},
	}
	body, _ := json.Marshal(bank)
	if err := os.WriteFile(filepath.Join(courseDir, questionBankFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	overview := Overview{
		Courses: []Course{{
			ID: "03-CF", Dir: "AI化教学创新03-CF", Path: courseDir, Exists: true, Type: "讲次制", Lectures: 2,
			Testbank: &Testbank{Title: "《C语言程序设计》分级测试题库", CertNames: []string{"a", "b", "c"},
				Levels: map[string]Level{"L1": {Pass: 60}, "L2": {Pass: 70}, "L3": {Pass: 80}}},
		}},
	}
	ob, _ := json.Marshal(overview)
	if err := os.WriteFile(filepath.Join(root, overviewFile), ob, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSyncWritesQuestions(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	if err := Run(context.Background(), database, buildQuestionFixture(t)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM question q JOIN course c ON c.id=q.course_id WHERE c.course_code='03-CF'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("应同步 2 题，实际 %d", count)
	}
	var qtype, grad, stem, answer string
	if err := database.QueryRow(`SELECT type, gradability, stem, answer FROM question q JOIN course c ON c.id=q.course_id WHERE c.course_code='03-CF' AND q.qno='L1-01-01'`).
		Scan(&qtype, &grad, &stem, &answer); err != nil {
		t.Fatal(err)
	}
	if qtype != "single" || grad != "auto" || stem != "赋值是表达式吗？" || answer != "A" {
		t.Fatalf("客观题字段错误: type=%s grad=%s stem=%s answer=%s", qtype, grad, stem, answer)
	}
	// 主观题：answer 取 answer_key，gradability=manual
	if err := database.QueryRow(`SELECT type, gradability, answer FROM question q JOIN course c ON c.id=q.course_id WHERE c.course_code='03-CF' AND q.qno='L2-02-01'`).
		Scan(&qtype, &grad, &answer); err != nil {
		t.Fatal(err)
	}
	if qtype != "short" || grad != "manual" || answer != "间接访问内存" {
		t.Fatalf("主观题字段错误: type=%s grad=%s answer=%s", qtype, grad, answer)
	}
}

func TestSyncKeepsOverrideQuestion(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildQuestionFixture(t)
	// 先跑一次，拿到 course_id，再把某题标记为平台改过（is_override=true + 改答案）。
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE question SET is_override = true, answer = '平台改的答案', stem = '平台改的题干' WHERE qno = 'L1-01-01'`); err != nil {
		t.Fatal(err)
	}
	// 再跑一次：平台改过的题不得被上游覆盖。
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	var stem, answer string
	if err := database.QueryRow(`SELECT stem, answer FROM question WHERE qno='L1-01-01'`).Scan(&stem, &answer); err != nil {
		t.Fatal(err)
	}
	if stem != "平台改的题干" || answer != "平台改的答案" {
		t.Fatalf("平台改过的题被 sync 覆盖: stem=%s answer=%s", stem, answer)
	}
}

func TestSyncPrunesRemovedQuestion(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildQuestionFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	// 上游删掉一题（只留 1 题）后重跑。
	bank := QuestionBank{Questions: []Question{
		{ID: "L1-01-01", Level: "L1", Lecture: "01", Type: "单选", Score: 1.5, Stem: "赋值是表达式吗？",
			Options: []string{"- A. 是"}, Answer: "A", AutoGradable: true},
	}}
	body, _ := json.Marshal(bank)
	if err := os.WriteFile(filepath.Join(root, "AI化教学创新03-CF", questionBankFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM question q JOIN course c ON c.id=q.course_id WHERE c.course_code='03-CF' AND q.is_override=false`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("上游删题后应剩 1 题，实际 %d", count)
	}
}
