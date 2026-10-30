package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

type aiCitation struct {
	LectureNo  int    `json:"lecture_no"`
	Title      string `json:"title"`
	StorageKey string `json:"storage_key,omitempty"`
}

type aiAnswer struct {
	Answer    string       `json:"answer"`
	Outcome   string       `json:"outcome"`
	Citations []aiCitation `json:"citations"`
	Model     string       `json:"model"`
	FilingNo  string       `json:"filing_no"`
	Remaining int          `json:"remaining"`
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

// askAI POST /ai/ask —— AI 助学 MVP：仅当前讲次资料、必须带引用、测评中关闭。
func (s *Server) askAI(c *gin.Context) {
	if s.cfg == nil || !s.cfg.LLMEnabled() || s.cfg.LLMFilingNo == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10601, Msg: "AI 助学尚未完成模型登记配置"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		CourseCode string `json:"course_code"`
		LectureNo  int    `json:"lecture_no"`
		Question   string `json:"question"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.CourseCode == "" || input.LectureNo < 1 || strings.TrimSpace(input.Question) == "" || len([]rune(input.Question)) > 1000 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "提问参数无效"})
		return
	}
	input.Question = strings.TrimSpace(input.Question)

	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id FROM course WHERE course_code=$1 AND deleted_at IS NULL`, input.CourseCode).Scan(&courseID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if !s.hasActiveEntitlement(c.Request.Context(), student.ID, courseID) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "解锁课程后才能使用 AI 助学"})
		return
	}
	var ongoing int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM exam_session WHERE student_id=$1 AND course_id=$2 AND status='ongoing'`,
		student.ID, courseID).Scan(&ongoing)
	if ongoing > 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10602, Msg: "测评进行中，AI 助学已关闭"})
		return
	}

	chunks, citations := s.courseContext(c.Request.Context(), courseID, input.LectureNo, input.Question)
	if len(chunks) == 0 {
		c.JSON(http.StatusOK, envelope{Code: 0, Data: aiAnswer{
			Answer:  "这门课的资料里没有这条依据，暂时无法回答。请先阅读本讲讲义或换一个与本讲相关的问题。",
			Outcome: "fallback", Citations: []aiCitation{}, Model: s.cfg.LLMModelName(), FilingNo: s.cfg.LLMFilingNo,
		}})
		return
	}
	remaining, err := consumeAIQuota(c.Request.Context(), s.dbConn, student.ID, courseID, s.cfg.AIDailyQuota)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, envelope{Code: 10603, Msg: "今日 AI 提问额度已用完"})
		return
	}

	started := time.Now()
	answer, tokensIn, tokensOut, err := s.callDeepSeek(c.Request.Context(), input.Question, chunks)
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 服务暂时不可用"})
		return
	}
	if strings.TrimSpace(answer) == "" {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 未返回有效内容"})
		return
	}
	// 引用是硬约束：模型没写来源时由服务端补齐，绝不返回无来源回答。
	if !strings.Contains(answer, "来源：") {
		var parts []string
		for _, cite := range citations {
			parts = append(parts, fmt.Sprintf("第%d讲《%s》", cite.LectureNo, cite.Title))
		}
		answer = strings.TrimSpace(answer) + "\n\n来源：" + strings.Join(parts, "；")
	}
	payload, _ := json.Marshal(gin.H{
		"model": s.cfg.LLMModelName(), "model_version": s.cfg.LLMModelName(),
		"filing_no": s.cfg.LLMFilingNo, "outcome": "answered",
		"citations": citations, "tokens_in": tokensIn, "tokens_out": tokensOut,
		"latency_ms": time.Since(started).Milliseconds(), "question": input.Question,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "student", ActorID: student.ID, Action: "ai.answer",
		TargetType: "course", TargetID: input.CourseCode, After: string(payload), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: aiAnswer{
		Answer: answer, Outcome: "answered", Citations: citations,
		Model: s.cfg.LLMModelName(), FilingNo: s.cfg.LLMFilingNo, Remaining: remaining,
	}})
}

func (s *Server) lectureContext(ctx context.Context, courseID int64, lectureNo int) (string, []aiCitation) {
	if s.cfg.ContentDir == "" {
		return "", nil
	}
	rows, err := s.dbConn.QueryContext(ctx,
		`SELECT l.no, l.title, a.title, a.storage_key, a.kind
		 FROM asset a JOIN lecture l ON l.id=a.lecture_id
		 WHERE l.course_id=$1 AND l.no=$2
		   AND a.kind IN ('markdown','html','code','frozen_output')
		 ORDER BY a.sort, a.id LIMIT 20`, courseID, lectureNo)
	if err != nil {
		return "", nil
	}
	defer rows.Close()
	var builder strings.Builder
	citations := make([]aiCitation, 0)
	for rows.Next() {
		var no int
		var lectureTitle, title, key, kind string
		if err := rows.Scan(&no, &lectureTitle, &title, &key, &kind); err != nil {
			continue
		}
		path := filepath.Join(s.cfg.ContentDir, filepath.FromSlash(key))
		cleanRoot := filepath.Clean(s.cfg.ContentDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(path), cleanRoot) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		text := string(data)
		if kind == "html" {
			text = htmlTagRE.ReplaceAllString(text, " ")
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if len([]rune(text)) > 4000 {
			text = string([]rune(text)[:4000])
		}
		fmt.Fprintf(&builder, "[L%d] %s / %s\n%s\n\n", no, lectureTitle, title, text)
		citations = append(citations, aiCitation{LectureNo: no, Title: title, StorageKey: key})
		if builder.Len() > 12000 {
			break
		}
	}
	return builder.String(), citations
}

type contextChunk struct {
	score    int
	text     string
	citation aiCitation
}

// courseContext 从全课程文本资产做轻量检索：当前讲次优先，其余按问题词命中排序。
func (s *Server) courseContext(ctx context.Context, courseID int64, lectureNo int, question string) (string, []aiCitation) {
	if s.cfg.ContentDir == "" {
		return "", nil
	}
	rows, err := s.dbConn.QueryContext(ctx,
		`SELECT l.no, l.title, a.title, a.storage_key, a.kind
		 FROM asset a JOIN lecture l ON l.id=a.lecture_id
		 WHERE l.course_id=$1 AND a.kind IN ('markdown','html','code','frozen_output')
		 ORDER BY l.no, a.sort, a.id`, courseID)
	if err != nil {
		return "", nil
	}
	defer rows.Close()
	tokens := questionTokens(question)
	chunks := make([]contextChunk, 0)
	for rows.Next() {
		var no int
		var lectureTitle, title, key, kind string
		if err := rows.Scan(&no, &lectureTitle, &title, &key, &kind); err != nil {
			continue
		}
		path := filepath.Join(s.cfg.ContentDir, filepath.FromSlash(key))
		cleanRoot := filepath.Clean(s.cfg.ContentDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(path), cleanRoot) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		text := string(data)
		if kind == "html" {
			text = htmlTagRE.ReplaceAllString(text, " ")
		}
		for _, part := range splitContext(strings.TrimSpace(text), 900) {
			if part == "" {
				continue
			}
			score := 0
			if no == lectureNo {
				score += 5
			}
			lower := strings.ToLower(part)
			for _, token := range tokens {
				if strings.Contains(lower, token) {
					score += 2
				}
			}
			chunks = append(chunks, contextChunk{
				score: score, text: part,
				citation: aiCitation{LectureNo: no, Title: title, StorageKey: key},
			})
		}
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].score > chunks[j].score })
	var builder strings.Builder
	citations := make([]aiCitation, 0)
	seen := map[string]bool{}
	for _, chunk := range chunks {
		if chunk.score <= 0 && len(citations) > 0 {
			continue
		}
		if len([]rune(builder.String()))+len([]rune(chunk.text)) > 12000 || len(citations) >= 8 {
			break
		}
		fmt.Fprintf(&builder, "[L%d] %s / %s\n%s\n\n", chunk.citation.LectureNo,
			chunk.citation.Title, chunk.citation.StorageKey, chunk.text)
		key := fmt.Sprintf("%d|%s", chunk.citation.LectureNo, chunk.citation.Title)
		if !seen[key] {
			seen[key] = true
			citations = append(citations, chunk.citation)
		}
	}
	return builder.String(), citations
}

func splitContext(text string, maxRunes int) []string {
	var out []string
	for _, paragraph := range strings.Split(text, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		runes := []rune(paragraph)
		for len(runes) > maxRunes {
			out = append(out, strings.TrimSpace(string(runes[:maxRunes])))
			runes = runes[maxRunes:]
		}
		if len(runes) > 0 {
			out = append(out, strings.TrimSpace(string(runes)))
		}
	}
	return out
}

func questionTokens(question string) []string {
	lower := strings.ToLower(question)
	words := regexp.MustCompile(`[a-z0-9_]{2,}`).FindAllString(lower, -1)
	han := make([]rune, 0)
	for _, r := range []rune(lower) {
		if unicode.Is(unicode.Han, r) {
			han = append(han, r)
		}
	}
	for i := 0; i+1 < len(han); i++ {
		words = append(words, string(han[i:i+2]))
	}
	return words
}

func consumeAIQuota(ctx context.Context, db *sql.DB, studentID, courseID int64, limit int) (int, error) {
	if limit <= 0 {
		limit = 20
	}
	date := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	var used int
	err := db.QueryRowContext(ctx,
		`INSERT INTO ai_daily_quota(student_id,course_id,stat_date,used) VALUES($1,$2,$3,1)
		 ON CONFLICT(student_id,course_id,stat_date) DO UPDATE SET used=ai_daily_quota.used+1
		 WHERE ai_daily_quota.used < $4 RETURNING used`,
		studentID, courseID, date, limit).Scan(&used)
	if err != nil {
		return 0, err
	}
	return limit - used, nil
}

func (s *Server) callDeepSeek(ctx context.Context, question, contextText string) (string, int, int, error) {
	system := `你是“第三课堂”的课程助教。必须遵守：
1. 只能依据下面提供的课程资料回答，不得使用外部知识补充事实；
2. 每条结论必须注明来源段落（形如“来源：第N讲《标题》”）；
3. 资料没有依据时直接说“这门课的资料里没有这条”，不要编造；
4. 与课程无关的问题礼貌拒答，并引导回本课程；
5. 不提供测评答案原文；测评期间不允许使用。
6. 用简洁中文回答，先给结论，再给依据。`
	body := map[string]any{
		"model": s.cfg.LLMModelName(),
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": "课程资料：\n" + contextText + "\n\n学员问题：" + question},
		},
		"temperature": 0.2,
		"max_tokens":  800,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(s.cfg.LLMBase(), "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.LLMKey())
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, 0, fmt.Errorf("deepseek status %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil || len(out.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("deepseek response invalid")
	}
	return out.Choices[0].Message.Content, out.Usage.PromptTokens, out.Usage.CompletionTokens, nil
}
