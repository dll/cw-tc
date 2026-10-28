package sync

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type lecturePrompt struct {
	key, prompt, answer string
}

var (
	// 题目标题：## 第1题 / ## 🤔 第1题 / ## 题 1 / ## 题一 / ## 题目1 / ## 思考题 ① / ## 1. / ## ①
	questionHeading = regexp.MustCompile(`^#{2,3}\s*(?:🤔|💭)?\s*(?:第\s*[0-9]+\s*题|思考题\s*(?:[0-9]+|[①②③④⑤⑥⑦⑧⑨⑩])|题\s*(?:[0-9]+|[一二三四五六七八九十]+)|题目\s*0*[0-9]+|[0-9]+\s*[.、：:]|[①②③④⑤⑥⑦⑧⑨⑩])`)
	// 加粗题号：**1. 题干**
	boldQuestion = regexp.MustCompile(`^\*\*\s*([0-9]+)\s*[.、]\s*(.+?)\s*\*\*\s*$`)
	// 集中给出参考答案的章节：## 参考答案 / ## 💡 参考思路 / ## 📖 参考答案
	collectHeading = regexp.MustCompile(`^##\s*(?:💡|📖)?\s*(?:参考思路|参考答案)`)
	// 集中答案里的单题标题：### 第1题 / ### 题 1 / ### 题目1
	collectItem = regexp.MustCompile(`^#{3,4}\s*(?:第\s*([0-9]+)\s*题|思考题\s*([0-9①②③④⑤⑥⑦⑧⑨⑩])|题\s*([0-9]+|[一二三四五六七八九十]+)|题目\s*0*([0-9]+))\s*[：:、.｜|\s]*(.*)$`)
)

var questionNumberPatterns = []*regexp.Regexp{
	regexp.MustCompile(`第\s*([0-9]+)\s*题`),
	regexp.MustCompile(`思考题\s*([0-9]+)`),
	regexp.MustCompile(`题目\s*0*([0-9]+)`),
	regexp.MustCompile(`题\s*([0-9]+)`),
	regexp.MustCompile(`题\s*([一二三四五六七八九十]+)`),
	regexp.MustCompile(`([0-9]+)\s*[.、：:]`),
}

// answerMarker 识别行内解答标记，并返回标记同一行后面的正文。
func answerMarker(line string) (string, bool) {
	prefixes := []string{
		"**完整参考答案**", "**参考答案**", "**参考要点**", "**怎么算答对", "**判据**", "**【解答】**", "**【答案】**",
		"### 【完整参考答案】", "### 【参考答案】", "### 【解答】", "### 【答案】",
		"### 完整参考答案", "### 参考答案", "### 解答", "### 答案",
		"**解答**", "**答案**", "**解答", "**答案",
		"【完整参考答案】", "【参考答案】", "【解答】", "【答案】",
		"<summary>参考答案</summary>", "<summary>参考要点</summary>",
	}
	for _, prefix := range prefixes {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		// 去掉 **解答（实测）** 这类限定语。
		if strings.HasPrefix(rest, "（") || strings.HasPrefix(rest, "(") {
			if end := strings.IndexAny(rest, "）)"); end >= 0 {
				_, size := utf8.DecodeRuneInString(rest[end:])
				rest = strings.TrimSpace(rest[end+size:])
			}
		}
		rest = strings.Trim(rest, "*")
		rest = strings.TrimSpace(rest)
		rest = strings.TrimPrefix(rest, "了")
		rest = strings.Trim(rest, "*")
		rest = strings.TrimSpace(rest)
		rest = strings.TrimPrefix(rest, "：")
		rest = strings.TrimPrefix(rest, ":")
		return strings.TrimSpace(rest), true
	}
	return "", false
}

// hintMarker 识别提示段落（不计入题干，也不当作参考答案）。
func hintMarker(line string) bool {
	for _, prefix := range []string{
		"### 【提示】", "### 提示", "**【提示】**", "**提示**", "【提示】", "**提示",
		"<summary>提示</summary>",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

var circledNumbers = []rune("①②③④⑤⑥⑦⑧⑨⑩")

// parseLecturePrompts 抽取明确标号的题目与参考答案。
// 支持上游实际存在的多种写法（行内解答 / 章节集中解答 / 提示段），无解答时保留开放题。
func parseLecturePrompts(body string) []lecturePrompt {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	collected, collectAt := collectReferenceAnswers(lines)

	items := make([]lecturePrompt, 0)
	var prompt, answer []string
	number := 0
	active, inAnswer, inHint := false, false, false

	flush := func() {
		if !active {
			return
		}
		p := strings.TrimSpace(strings.Join(prompt, "\n"))
		a := strings.TrimSpace(strings.Join(answer, "\n"))
		if a == "" && number > 0 {
			a = collected[number]
		}
		if p != "" {
			items = append(items, lecturePrompt{key: strconv.Itoa(len(items) + 1), prompt: p, answer: a})
		}
		prompt, answer = nil, nil
		number = 0
		active, inAnswer, inHint = false, false, false
	}

	for idx, line := range lines {
		if collectAt >= 0 && idx >= collectAt {
			break
		}
		trim := strings.TrimSpace(line)
		if questionHeading.MatchString(trim) {
			flush()
			active = true
			number = numberInQuestion(trim)
			prompt = append(prompt, cleanQuestionHeading(trim))
			continue
		}
		// 加粗题号只在题目区识别，避免把解答里的 **1. ...** 列表误当成新题。
		if !inAnswer && !inHint {
			if m := boldQuestion.FindStringSubmatch(trim); m != nil {
				flush()
				active = true
				number, _ = strconv.Atoi(m[1])
				prompt = append(prompt, strings.TrimSpace(m[2]))
				continue
			}
		}
		if !active {
			continue
		}
		if rest, ok := answerMarker(trim); ok {
			inAnswer, inHint = true, false
			if rest != "" {
				answer = append(answer, rest)
			}
			continue
		}
		if hintMarker(trim) {
			inHint, inAnswer = true, false
			continue
		}
		if strings.HasPrefix(trim, "## ") {
			flush()
			continue
		}
		switch {
		case inAnswer:
			answer = append(answer, line)
		case inHint:
			// 提示既不是题干也不是参考答案，跳过
		default:
			prompt = append(prompt, line)
		}
	}
	flush()
	return items
}

// collectReferenceAnswers 解析「## 参考答案 / ## 参考思路」章节里按题号分组的答案。
func collectReferenceAnswers(lines []string) (map[int]string, int) {
	for i, line := range lines {
		if !collectHeading.MatchString(strings.TrimSpace(line)) {
			continue
		}
		answers := make(map[int]string)
		var number int
		var buf []string
		flush := func() {
			if number > 0 {
				if text := cleanAnswerText(strings.Join(buf, "\n")); text != "" {
					answers[number] = text
				}
			}
			buf = nil
		}
		for j := i + 1; j < len(lines); j++ {
			trim := strings.TrimSpace(lines[j])
			if strings.HasPrefix(trim, "## ") {
				flush()
				return answers, i
			}
			if m := collectItem.FindStringSubmatch(trim); m != nil {
				flush()
				number = itemNumber(m)
				continue
			}
			if number > 0 {
				buf = append(buf, lines[j])
			}
		}
		flush()
		return answers, i
	}
	return nil, -1
}

func numberInQuestion(line string) int {
	for _, pattern := range questionNumberPatterns {
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if n, err := strconv.Atoi(match[1]); err == nil {
			return n
		}
		return chineseNumber(match[1])
	}
	for i, r := range circledNumbers {
		if strings.ContainsRune(line, r) {
			return i + 1
		}
	}
	return 0
}

func itemNumber(match []string) int {
	for _, group := range match[1:5] {
		if group == "" {
			continue
		}
		if n, err := strconv.Atoi(group); err == nil {
			return n
		}
		if n := circledNumber(group); n > 0 {
			return n
		}
		return chineseNumber(group)
	}
	return 0
}

func circledNumber(text string) int {
	for i, r := range circledNumbers {
		if string(r) == text {
			return i + 1
		}
	}
	return 0
}

var chineseDigits = map[rune]int{
	'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
	'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
}

// chineseNumber 解析「一」~「九十九」这类常见题号。
func chineseNumber(text string) int {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return 0
	}
	if len(runes) == 1 {
		return chineseDigits[runes[0]]
	}
	if runes[0] == '十' {
		return 10 + chineseDigits[runes[1]]
	}
	if len(runes) == 2 && runes[1] == '十' {
		return chineseDigits[runes[0]] * 10
	}
	if len(runes) == 3 && runes[1] == '十' {
		return chineseDigits[runes[0]]*10 + chineseDigits[runes[2]]
	}
	return 0
}

func cleanQuestionHeading(line string) string {
	text := strings.TrimSpace(strings.TrimPrefix(line, "##"))
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "🤔")
	text = strings.TrimPrefix(text, "💭")
	return strings.TrimSpace(text)
}

func cleanAnswerText(text string) string {
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "<details>" || trim == "</details>" || trim == "---" {
			continue
		}
		if strings.HasPrefix(trim, "<summary>") && strings.HasSuffix(trim, "</summary>") {
			continue
		}
		if strings.HasPrefix(trim, "**参考答案**") || strings.HasPrefix(trim, "**解答**") || strings.HasPrefix(trim, "**答案**") {
			trim = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(trim, "**参考答案**"), "**解答**"), "**答案**"))
			trim = strings.TrimPrefix(trim, "：")
			trim = strings.TrimPrefix(trim, ":")
			if trim == "" {
				continue
			}
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// syncLectureQuestions 仅同步上游讲次的思考题，来源精确到文件；平台讲次不参与。
func syncLectureQuestions(ctx context.Context, database *sql.DB, course Course) error {
	entries, err := os.ReadDir(course.Path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		match := lectureDirRe.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		no, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		var lectureID int64
		err = database.QueryRowContext(ctx, `SELECT l.id FROM lecture l JOIN course c ON c.id=l.course_id WHERE c.course_code=$1 AND c.source='upstream' AND l.no=$2 AND l.origin='upstream'`, course.ID, no).Scan(&lectureID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		path := filepath.Join(course.Path, entry.Name(), "思考题.md")
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		prompts := parseLecturePrompts(string(body))
		for _, q := range prompts {
			_, err = database.ExecContext(ctx, `INSERT INTO lecture_question(lecture_id,source_key,prompt,reference_answer,evidence_ref) VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(lecture_id,source_key) DO UPDATE SET prompt=excluded.prompt,reference_answer=excluded.reference_answer,evidence_ref=excluded.evidence_ref`,
				lectureID, q.key, q.prompt, nullIfEmpty(q.answer), filepath.ToSlash(filepath.Join(course.Dir, entry.Name(), "思考题.md")))
			if err != nil {
				return fmt.Errorf("同步第 %d 讲思考题失败: %w", no, err)
			}
		}
		if len(prompts) == 0 {
			_, err = database.ExecContext(ctx, `DELETE FROM lecture_question WHERE lecture_id=$1`, lectureID)
		} else {
			_, err = database.ExecContext(ctx, `DELETE FROM lecture_question WHERE lecture_id=$1 AND CAST(source_key AS INTEGER)>$2`, lectureID, len(prompts))
		}
		if err != nil {
			return err
		}
	}
	return nil
}
