package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/aigateway"
	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// myLearningPlan GET /me/learning-plan?course_code= —— AI 学习路径（只基于本课进度 / 成绩 / 课程资料）。
func (s *Server) myLearningPlan(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	code := strings.TrimSpace(c.Query("course_code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "course_code 必填"})
		return
	}
	gateway := s.aiGateway()
	if gateway == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10601, Msg: "AI 学习路径未配置模型备案号"})
		return
	}
	var courseID int64
	var courseName string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, name FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).
		Scan(&courseID, &courseName); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if !s.hasActiveEntitlement(c.Request.Context(), student.ID, courseID) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "解锁课程后才能生成学习路径"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT l.no, l.title, CASE WHEN p.completed_at IS NOT NULL THEN 1 ELSE 0 END,
		        COALESCE(p.last_studied_at,'')
		 FROM lecture l LEFT JOIN lecture_progress p ON p.lecture_id = l.id AND p.student_id = $1
		 WHERE l.course_id = $2 ORDER BY l.no`, student.ID, courseID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "学习数据读取失败"})
		return
	}
	defer rows.Close()
	type lectureStatus struct {
		No        int    `json:"no"`
		Title     string `json:"title"`
		Completed bool   `json:"completed"`
		LastAt    string `json:"last_studied_at,omitempty"`
	}
	lectures := make([]lectureStatus, 0)
	lastLecture := 1
	for rows.Next() {
		var item lectureStatus
		var completed int
		if err := rows.Scan(&item.No, &item.Title, &completed, &item.LastAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "学习数据读取失败"})
			return
		}
		item.Completed = completed == 1
		lectures = append(lectures, item)
	}
	if len(lectures) == 0 {
		c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"outcome": "empty", "message": "本课还没有讲次"}})
		return
	}
	var latestScore sql.NullFloat64
	var latestLevel, latestBand sql.NullString
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT CAST(total_score AS REAL), level, band FROM exam_result
		 WHERE student_id = $1 AND course_id = $2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		student.ID, courseID).Scan(&latestScore, &latestLevel, &latestBand)
	contextText, citations := s.courseContext(c.Request.Context(), courseID, lastLecture, "重点 复习 学习路径")
	if len(citations) == 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10601, Msg: "本课暂无可引用的课程资料，无法生成有据的学习路径"})
		return
	}
	remaining, err := consumeAIQuota(c.Request.Context(), s.dbConn, student.ID, courseID, s.cfg.AIDailyQuota)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, envelope{Code: 10603, Msg: "今日 AI 使用额度已用完"})
		return
	}
	progressJSON, _ := json.Marshal(lectures)
	scoreText := "暂无测评成绩"
	if latestScore.Valid {
		scoreText = fmt.Sprintf("最近一次测评：%s 分（等级 %s，档位 %s）", fmtFloat(latestScore.Float64), latestLevel.String, latestBand.String)
	}
	prompt := fmt.Sprintf("课程：%s\n讲次进度：%s\n%s\n课程资料：\n%s", courseName, string(progressJSON), scoreText, contextText)
	resp, err := gateway.Complete(c.Request.Context(), "assistant.strong", aigateway.Request{
		System: "你是课程学习规划师。只依据给定的讲次进度、成绩与课程资料制定计划；不得编造资料或保证通过；给出未来 3 步具体行动，并注明依据讲次。",
		Prompt: prompt, Temperature: 0.2, MaxTokens: 600,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 学习路径生成失败"})
		return
	}
	after, _ := json.Marshal(gin.H{
		"alias": resp.Alias, "model": resp.Model, "model_version": resp.ModelVersion,
		"filing_no": resp.FilingNo, "tokens_in": resp.TokensIn, "tokens_out": resp.TokensOut,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "student", ActorID: student.ID, Action: "ai.learning_plan",
		TargetType: "course", TargetID: code, After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"plan": resp.Content, "citations": citations, "remaining": remaining,
		"alias": resp.Alias, "model": resp.Model, "filing_no": resp.FilingNo,
		"note": "计划由 AI 辅助生成，请结合自身情况调整；不是测评答案或通过承诺",
		"at":   time.Now().UTC().Format(time.RFC3339),
	}})
}

func fmtFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}
