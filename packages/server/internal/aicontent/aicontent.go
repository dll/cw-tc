// Package aicontent 负责 AI 生成草稿的解析、自动体检与内容指纹（PRD §7.16 P-2.1 / P-2.2）。
package aicontent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	MinBodyChars  = 300
	MaxLectures   = 5
	AILabelPrefix = "> AI 辅助生成，经教师与管理员审核后上线\n\n"
)

var (
	ErrParse   = errors.New("AI 草稿不是合法 JSON 或结构不完整")
	ErrCount   = errors.New("讲次数量与声明不一致")
	ErrTooMany = errors.New("单次生成讲次数超过上限")
)

type Lecture struct {
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

func ParseLectures(raw string, expected int) ([]Lecture, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	var payload struct {
		Lectures []Lecture `json:"lectures"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		// 兼容模型直接返回数组。
		var direct []Lecture
		if err2 := json.Unmarshal([]byte(text), &direct); err2 != nil {
			return nil, ErrParse
		}
		payload.Lectures = direct
	}
	if len(payload.Lectures) == 0 || len(payload.Lectures) > MaxLectures {
		return nil, ErrTooMany
	}
	if expected > 0 && len(payload.Lectures) != expected {
		return nil, ErrCount
	}
	for i := range payload.Lectures {
		payload.Lectures[i].Title = strings.TrimSpace(payload.Lectures[i].Title)
		payload.Lectures[i].ContentMD = strings.TrimSpace(payload.Lectures[i].ContentMD)
		if payload.Lectures[i].Title == "" || payload.Lectures[i].ContentMD == "" {
			return nil, ErrParse
		}
	}
	return payload.Lectures, nil
}

// WithAILabel 在正文前加显式 AI 标识（学员端可见，不得刻意缩小/模糊）。
func WithAILabel(lectures []Lecture) []Lecture {
	out := make([]Lecture, len(lectures))
	for i, lecture := range lectures {
		out[i] = Lecture{Title: lecture.Title, ContentMD: AILabelPrefix + lecture.ContentMD}
	}
	return out
}

// AutoCheck 自动体检：结构 / 长度 / 引用 / 内容安全 / 去重；只挡明显不可用的草稿，不能替代双审。
func AutoCheck(lectures []Lecture, citations int) (bool, map[string]any) {
	report := map[string]any{
		"structure_ok": false, "length_ok": false, "citations_ok": citations > 0,
		"safe_ok": true, "dedup_ok": true,
	}
	if len(lectures) == 0 || len(lectures) > MaxLectures {
		return false, report
	}
	structure, length, safe, dedup := true, true, true, true
	seenTitle := map[string]bool{}
	seenContent := map[string]bool{}
	forbidden := []string{"标准答案", "考试答案", "测评原题", "包过", "保过"}
	for _, lecture := range lectures {
		if strings.TrimSpace(lecture.Title) == "" || strings.TrimSpace(lecture.ContentMD) == "" {
			structure = false
		}
		content := strings.TrimSpace(lecture.ContentMD)
		if len([]rune(content)) < MinBodyChars {
			length = false
		}
		for _, word := range forbidden {
			if strings.Contains(content, word) {
				safe = false
			}
		}
		if seenTitle[lecture.Title] || seenContent[content] {
			dedup = false
		}
		seenTitle[lecture.Title] = true
		seenContent[content] = true
	}
	report["structure_ok"] = structure
	report["length_ok"] = length
	report["safe_ok"] = safe
	report["dedup_ok"] = dedup
	report["lecture_count"] = len(lectures)
	report["min_chars"] = MinBodyChars
	return structure && length && citations > 0 && safe && dedup, report
}

// ContentMD 把多讲草稿拼成一份可预览 / 导出的 Markdown。
func ContentMD(lectures []Lecture) string {
	var builder strings.Builder
	for i, lecture := range lectures {
		if i > 0 {
			builder.WriteString("\n\n---\n\n")
		}
		builder.WriteString("# ")
		builder.WriteString(lecture.Title)
		builder.WriteString("\n\n")
		builder.WriteString(lecture.ContentMD)
	}
	return builder.String()
}

// Hash 计算当前版本草稿指纹（审的是哪一版）。
func Hash(lectures []Lecture) string {
	raw, _ := json.Marshal(lectures)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
