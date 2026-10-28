package api

import (
	"math"
	"strings"
	"unicode"
)

// gradeStopTerms 过滤无区分度的高频二元组，避免"因为/可以"等词抬高分数。
var gradeStopTerms = map[string]struct{}{
	"因为": {}, "所以": {}, "可以": {}, "就是": {}, "这个": {}, "一个": {},
	"我们": {}, "你们": {}, "他们": {}, "如果": {}, "但是": {}, "而且": {},
	"或者": {}, "不是": {}, "没有": {}, "对于": {}, "进行": {}, "需要": {},
	"应该": {}, "问题": {}, "答案": {}, "什么": {}, "怎么": {}, "这样": {},
	"那样": {}, "以及": {}, "同时": {}, "然后": {}, "其中": {}, "通过": {},
	"使用": {}, "例如": {}, "比如": {}, "主要": {}, "方面": {}, "情况": {},
}

// gradeLectureAnswer 依据参考答案给出 0~100 的参考相似度。
// 仅当参考答案含足够多可比较的关键词时才判分；开放题返回 graded=false。
func gradeLectureAnswer(answer, reference string) (int, bool) {
	terms := significantTerms(reference)
	if len(terms) < 3 {
		return 0, false
	}
	normalizedAnswer := normalizeForGrade(answer)
	if normalizedAnswer == "" {
		return 0, true
	}
	hits := 0
	for term := range terms {
		if strings.Contains(normalizedAnswer, term) {
			hits++
		}
	}
	score := int(math.Round(float64(hits) * 100 / float64(len(terms))))
	if len([]rune(strings.TrimSpace(answer))) < 10 && score > 30 {
		score = 30
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, true
}

// significantTerms 提取参考答案中的关键比较单元：
// 中文按二元组、英文/标识符按整词、数字按连续串。
func significantTerms(text string) map[string]struct{} {
	terms := make(map[string]struct{})
	runes := []rune(strings.ToLower(text))
	var hanRun, wordRun []rune
	flushHan := func() {
		for i := 0; i+1 < len(hanRun); i++ {
			term := string(hanRun[i : i+2])
			if _, stop := gradeStopTerms[term]; stop {
				continue
			}
			terms[term] = struct{}{}
		}
		hanRun = hanRun[:0]
	}
	flushWord := func() {
		if len(wordRun) >= 3 {
			terms[string(wordRun)] = struct{}{}
		}
		wordRun = wordRun[:0]
	}
	for _, r := range runes {
		switch {
		case unicode.Is(unicode.Han, r):
			flushWord()
			hanRun = append(hanRun, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushHan()
			wordRun = append(wordRun, r)
		default:
			flushHan()
			flushWord()
		}
	}
	flushHan()
	flushWord()
	return terms
}

func normalizeForGrade(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
