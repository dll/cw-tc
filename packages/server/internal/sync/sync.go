// Package sync 实现上游课程内容的只读同步。
//
// 铁律（AGENTS.md 铁律 1 / 铁律 12）：
//   - 只写 `source='upstream'` 的课程，绝不触碰 `platform` 课程；
//   - 只写 `origin='upstream'` 的讲次，绝不触碰 `ai_assisted` / `platform` 讲次；
//   - 只读覆盖：上游删掉的课/讲次会被清理，但平台自有数据（课程码/专业/价格/权益/订单）绝不写。
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Overview 是「课程总览.json」的顶层结构。
type Overview struct {
	Courses []Course `json:"courses"`
}

// Course 是「课程总览.json」里单门课的字段（与 PRD §6.4 一致）。
type Course struct {
	ID       string    `json:"id"`
	Dir      string    `json:"dir"`
	Path     string    `json:"path"`
	Exists   bool      `json:"exists"`
	Type     string    `json:"type"`
	Lectures int       `json:"lectures"`
	Outline  *string   `json:"outline"`
	Testbank *Testbank `json:"testbank"`
}

// Testbank 是课程题库的机器可读元数据。
type Testbank struct {
	Title      string           `json:"title"`
	CertPrefix string           `json:"cert_prefix"`
	CertNames  []string         `json:"cert_names"`
	Levels     map[string]Level `json:"levels"`
}

// Level 是分级测评的档位元数据。
type Level struct {
	Name        string `json:"name"`
	Certificate string `json:"certificate"`
	Count       int    `json:"count"`
	Pass        int    `json:"pass"`
	Good        int    `json:"good"`
	Excellent   int    `json:"excellent"`
}

// QuestionBank 是「测试题集/题库.json」的顶层结构。
type QuestionBank struct {
	Questions []Question `json:"questions"`
}

// Question 是题库里的一道题（上游只读，平台通过 is_override 覆盖）。
type Question struct {
	ID           string   `json:"id"`
	Level        string   `json:"level"`
	Lecture      string   `json:"lecture"`
	Type         string   `json:"type"`
	Score        float64  `json:"score"`
	Stem         string   `json:"stem"`
	Options      []string `json:"options"`
	Answer       string   `json:"answer"`
	Explanation  string   `json:"explanation"`
	AnswerKey    string   `json:"answer_key"`
	Scoring      string   `json:"scoring"`
	AutoGradable bool     `json:"auto_gradable"`
}

const overviewFile = "课程总览.json"
const questionBankFile = "测试题集/题库.json"

var lectureDirRe = regexp.MustCompile(`^(\d{1,2})_?(.+)$`)

// typeMap 上游中文题型 → 平台枚举（数据字典 §1 question.type）。
var typeMap = map[string]string{
	"单选":     "single",
	"判断":     "judge",
	"填空":     "blank",
	"名词解释":   "term",
	"简答":     "short",
	"计算":     "calc",
	"代码阅读":   "code_read",
	"SQL 阅读": "code_read",
	"工件阅读":   "code_read",
	"编程":     "code_read",
	"实验设计":   "design",
	"系统设计":   "design",
	"对比分析":   "compare",
	"故障归因":   "fault",
	"选型评审":   "review",
	"综合论证":   "synth",
}

// LoadOverview 从源目录读取并解析「课程总览.json」。
func LoadOverview(sourceDir string) (*Overview, error) {
	path := filepath.Join(sourceDir, overviewFile)
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	var overview Overview
	if err := json.Unmarshal(body, &overview); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	return &overview, nil
}

// Run 执行一次同步：上游课程与讲次只读覆盖，平台数据不动。
func Run(ctx context.Context, database *sql.DB, sourceDir string) error {
	if database == nil {
		return fmt.Errorf("数据库未配置")
	}
	overview, err := LoadOverview(sourceDir)
	if err != nil {
		return err
	}
	seenCourses := make([]string, 0, len(overview.Courses))
	for _, course := range overview.Courses {
		if !course.Exists || (course.Type != "讲次制" && course.Type != "案例制") {
			continue
		}
		if err := upsertCourse(ctx, database, course); err != nil {
			return err
		}
		seenCourses = append(seenCourses, course.ID)
		if course.Type == "讲次制" {
			if err := syncLectures(ctx, database, course); err != nil {
				return err
			}
		}
		if err := syncQuestions(ctx, database, course); err != nil {
			return err
		}
	}
	if err := pruneUpstreamCourses(ctx, database, seenCourses); err != nil {
		return err
	}
	return nil
}

func courseName(title string) string {
	name := strings.TrimSpace(title)
	name = strings.ReplaceAll(name, "《", "")
	name = strings.ReplaceAll(name, "》", "")
	name = strings.TrimSuffix(name, "分级测试题库")
	return strings.TrimSpace(name)
}

func upsertCourse(ctx context.Context, database *sql.DB, course Course) error {
	name := courseName(course.Testbank.Title)
	line := "aux"
	if course.Type == "讲次制" {
		line = "main"
	}
	certNames := buildCertNames(course.Testbank)
	certPassLines := buildCertPassLines(course.Testbank)
	_, err := database.ExecContext(ctx,
		`INSERT INTO course(course_code, source, name, line, lecture_count, dir, cert_names, cert_pass_lines, status) VALUES(?,?,?,?,?,?,?,?,'published')
		 ON CONFLICT(course_code) DO UPDATE SET source='upstream', name=excluded.name, line=excluded.line, lecture_count=excluded.lecture_count, dir=excluded.dir, cert_names=excluded.cert_names, cert_pass_lines=excluded.cert_pass_lines`,
		course.ID, "upstream", name, line, course.Lectures, course.Dir, certNames, certPassLines)
	return err
}

func buildCertNames(tb *Testbank) string {
	names := map[string]string{}
	order := []string{"L1", "L2", "L3"}
	for i, level := range order {
		if i < len(tb.CertNames) {
			names[level] = tb.CertNames[i]
		}
	}
	body, _ := json.Marshal(names)
	return string(body)
}

func buildCertPassLines(tb *Testbank) string {
	lines := map[string]int{}
	for level, info := range tb.Levels {
		lines[level] = info.Pass
	}
	body, _ := json.Marshal(lines)
	return string(body)
}

func syncLectures(ctx context.Context, database *sql.DB, course Course) error {
	// 1. 找到（或新建）该 upstream 课程的 course_id。
	var courseID int64
	if err := database.QueryRowContext(ctx, `SELECT id FROM course WHERE course_code = ? AND source = 'upstream'`, course.ID).Scan(&courseID); err != nil {
		return fmt.Errorf("课程 %s 未找到: %w", course.ID, err)
	}

	// 2. 列出讲次目录（nn_标题），按编号升序。
	entries, err := os.ReadDir(course.Path)
	if err != nil {
		return fmt.Errorf("读取课程目录 %s 失败: %w", course.Path, err)
	}
	type lecture struct {
		no    int
		title string
	}
	lectures := make([]lecture, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m := lectureDirRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		no, err := strconv.Atoi(m[1])
		if err != nil || no < 1 {
			continue
		}
		lectures = append(lectures, lecture{no: no, title: m[2]})
	}
	sort.Slice(lectures, func(i, j int) bool { return lectures[i].no < lectures[j].no })

	// 3. upsert 讲次（origin='upstream'），并清理上游已删讲次。
	seen := make([]int, 0, len(lectures))
	for _, lec := range lectures {
		if _, err := database.ExecContext(ctx,
			`INSERT INTO lecture(course_id, no, title, origin) VALUES(?,?,?,'upstream')
			 ON CONFLICT(course_id, no) DO UPDATE SET title=excluded.title, origin='upstream'`,
			courseID, lec.no, lec.title); err != nil {
			return fmt.Errorf("写入讲次 %d 失败: %w", lec.no, err)
		}
		seen = append(seen, lec.no)
	}
	return pruneUpstreamLectures(ctx, database, courseID, seen)
}

// pruneUpstreamCourses 删除上游已不再存在的课程（只动 source='upstream'），先删其上游讲次再删课程（避免外键）。
func pruneUpstreamCourses(ctx context.Context, database *sql.DB, keep []string) error {
	notIn := "1=1"
	args := make([]any, 0, len(keep))
	if len(keep) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",")
		notIn = "course_code NOT IN (" + placeholders + ")"
		for _, code := range keep {
			args = append(args, code)
		}
	}
	if _, err := database.ExecContext(ctx,
		`DELETE FROM lecture WHERE origin='upstream' AND course_id IN (SELECT id FROM course WHERE source='upstream' AND `+notIn+`)`, args...); err != nil {
		return err
	}
	_, err := database.ExecContext(ctx, `DELETE FROM course WHERE source='upstream' AND `+notIn, args...)
	return err
}

// pruneUpstreamLectures 删除上游已删讲次（只动 origin='upstream'，绝不碰 ai_assisted/platform）。
func pruneUpstreamLectures(ctx context.Context, database *sql.DB, courseID int64, keep []int) error {
	if len(keep) == 0 {
		_, err := database.ExecContext(ctx, `DELETE FROM lecture WHERE course_id = ? AND origin = 'upstream'`, courseID)
		return err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",")
	args := make([]any, 0, len(keep)+1)
	args = append(args, courseID)
	for _, no := range keep {
		args = append(args, no)
	}
	_, err := database.ExecContext(ctx,
		`DELETE FROM lecture WHERE course_id = ? AND origin = 'upstream' AND no NOT IN (`+placeholders+`)`, args...)
	return err
}

// syncQuestions 读取该课程「测试题集/题库.json」，只写 upstream 课程的题目，不覆盖平台改过的题（is_override=true）。
func syncQuestions(ctx context.Context, database *sql.DB, course Course) error {
	path := filepath.Join(course.Path, questionBankFile)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 无题库的课程（如部分案例制）跳过
		}
		return fmt.Errorf("读取题库 %s 失败: %w", path, err)
	}
	var bank QuestionBank
	if err := json.Unmarshal(body, &bank); err != nil {
		return fmt.Errorf("解析题库 %s 失败: %w", path, err)
	}
	var courseID int64
	if err := database.QueryRowContext(ctx, `SELECT id FROM course WHERE course_code = ? AND source = 'upstream'`, course.ID).Scan(&courseID); err != nil {
		return fmt.Errorf("课程 %s 未找到: %w", course.ID, err)
	}
	seen := make([]string, 0, len(bank.Questions))
	for _, q := range bank.Questions {
		gradability := "manual"
		if q.AutoGradable {
			gradability = "auto"
		}
		answer := q.Answer
		if answer == "" {
			answer = q.AnswerKey
		}
		if _, err := database.ExecContext(ctx,
			`INSERT INTO question(course_id, qno, level, type, gradability, stem, options, answer, analysis, rubric, score, evidence, is_override)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,false)
			 ON CONFLICT(course_id, qno) DO UPDATE SET
			   level=excluded.level, type=excluded.type, gradability=excluded.gradability,
			   stem=excluded.stem, options=excluded.options, answer=excluded.answer,
			   analysis=excluded.analysis, rubric=excluded.rubric, score=excluded.score, evidence=excluded.evidence
			 WHERE question.is_override = false`,
			courseID, q.ID, q.Level, questionType(q.Type), gradability, q.Stem,
			optionsJSON(q.Options), nullIfEmpty(answer), nullIfEmpty(q.Explanation),
			nullIfEmpty(q.Scoring), q.Score, evidenceJSON(q.Lecture)); err != nil {
			return fmt.Errorf("写入题目 %s 失败: %w", q.ID, err)
		}
		seen = append(seen, q.ID)
	}
	return pruneUpstreamQuestions(ctx, database, courseID, seen)
}

// pruneUpstreamQuestions 删除上游已删题目（只动 is_override=false，绝不碰平台改过的题）。
func pruneUpstreamQuestions(ctx context.Context, database *sql.DB, courseID int64, keep []string) error {
	if len(keep) == 0 {
		_, err := database.ExecContext(ctx, `DELETE FROM question WHERE course_id = ? AND is_override = false`, courseID)
		return err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",")
	args := make([]any, 0, len(keep)+1)
	args = append(args, courseID)
	for _, qno := range keep {
		args = append(args, qno)
	}
	_, err := database.ExecContext(ctx,
		`DELETE FROM question WHERE course_id = ? AND is_override = false AND qno NOT IN (`+placeholders+`)`, args...)
	return err
}

// questionType 把上游中文题型映射到平台枚举；未知题型按客观/主观回退。
func questionType(chinese string) string {
	if t, ok := typeMap[chinese]; ok {
		return t
	}
	return "short"
}

// optionsJSON 清洗选项（去掉 "- " 前缀）后序列化为 JSON 数组；空则返回 nil。
func optionsJSON(options []string) any {
	if len(options) == 0 {
		return nil
	}
	cleaned := make([]string, len(options))
	for i, opt := range options {
		cleaned[i] = strings.TrimPrefix(strings.TrimSpace(opt), "- ")
	}
	body, _ := json.Marshal(cleaned)
	return string(body)
}

// evidenceJSON 构造证据链（题目来源讲次）；空则返回 nil。
func evidenceJSON(lecture string) any {
	if lecture == "" {
		return nil
	}
	return fmt.Sprintf(`{"source":"L%s"}`, lecture)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
