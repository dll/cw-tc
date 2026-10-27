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
)

type lecturePrompt struct {
	key, prompt, answer string
}

var numberedHeading = regexp.MustCompile(`^##\s+(?:(?:[🤔💭]\s*)?(?:第\s*)?[0-9]+\s*(?:题|[.、：:])|(?:思考题\s*)?[①②③④⑤⑥⑦⑧⑨⑩])`)
var boldNumbered = regexp.MustCompile(`^\*\*[0-9]+[.、]\s*`)
var answerHeading = regexp.MustCompile(`^\*\*(?:解答|答案|参考答案)\*\*|^###\s*(?:解答|答案|参考答案)`)

// parseLecturePrompts 只抽取明确标号的题目。无解答时保留开放题，不虚构标准答案。
func parseLecturePrompts(body string) []lecturePrompt {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	items := make([]lecturePrompt, 0)
	var stem, answer []string
	active, inAnswer := false, false
	flush := func() {
		if !active {
			return
		}
		prompt := strings.TrimSpace(strings.Join(stem, "\n"))
		if prompt != "" {
			items = append(items, lecturePrompt{key: strconv.Itoa(len(items) + 1), prompt: prompt, answer: strings.TrimSpace(strings.Join(answer, "\n"))})
		}
		stem, answer, active, inAnswer = nil, nil, false, false
	}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if numberedHeading.MatchString(trim) || boldNumbered.MatchString(trim) {
			flush()
			active = true
			stem = append(stem, trim)
			continue
		}
		if !active {
			continue
		}
		if answerHeading.MatchString(trim) {
			inAnswer = true
			continue
		}
		if strings.HasPrefix(trim, "## ") && !inAnswer {
			flush()
			continue
		}
		if inAnswer {
			answer = append(answer, line)
		} else {
			stem = append(stem, line)
		}
	}
	flush()
	return items
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
