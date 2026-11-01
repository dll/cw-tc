package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/aigateway"
	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// teacherAIGradeSuggest POST /teacher/grade/:resultID/questions/:questionID/ai-suggest
//
// AI 只给“建议分数 + 采点理由”，**不写库、不判负**；最终分仍由教师提交。
func (s *Server) teacherAIGradeSuggest(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	resultID, err1 := strconv.ParseInt(c.Param("resultID"), 10, 64)
	questionID, err2 := strconv.ParseInt(c.Param("questionID"), 10, 64)
	if err1 != nil || err2 != nil || resultID <= 0 || questionID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	gateway := s.aiGateway()
	if gateway == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10601, Msg: "AI 阅卷辅助未配置模型备案号"})
		return
	}
	var courseID, sessionID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT course_id, exam_session_id FROM exam_result WHERE id = $1`, resultID).Scan(&courseID, &sessionID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "成绩记录不存在"})
		return
	}
	var owner sql.NullInt64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT teacher_id FROM course WHERE id = $1`, courseID).Scan(&owner); err != nil || !owner.Valid || owner.Int64 != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权批改该课程"})
		return
	}
	var qno, stem, reference string
	var maxScore float64
	var answersRaw string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT q.qno, q.stem, COALESCE(q.answer,''), CAST(q.score AS REAL), COALESCE(es.answers,'{}')
		 FROM question q JOIN exam_session es ON es.id = $1
		 WHERE q.id = $2 AND q.course_id = $3`, sessionID, questionID, courseID).
		Scan(&qno, &stem, &reference, &maxScore, &answersRaw); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "题目或答卷不存在"})
		return
	}
	var answers map[string]string
	_ = json.Unmarshal([]byte(answersRaw), &answers)
	studentAnswer := strings.TrimSpace(answers[qno])
	if studentAnswer == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "该题学员未作答"})
		return
	}
	prompt := fmt.Sprintf("题目：%s\n满分：%.1f 分\n参考答案（可能为空）：%s\n学员作答：%s\n请给出建议分数与采点理由。",
		stem, maxScore, reference, studentAnswer)
	resp, err := gateway.Complete(c.Request.Context(), "assistant.strong", aigateway.Request{
		System:      "你是课程阅卷辅助。只依据题目、参考答案与学员作答给建议分（0~满分）；必须逐点说明给分/扣分理由；不得声称最终判定，不得编造参考答案。",
		Prompt:      prompt,
		Temperature: 0.1,
		MaxTokens:   500,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 10602, Msg: "AI 阅卷建议失败"})
		return
	}
	after, _ := json.Marshal(gin.H{
		"alias": resp.Alias, "model": resp.Model, "model_version": resp.ModelVersion,
		"filing_no": resp.FilingNo, "tokens_in": resp.TokensIn, "tokens_out": resp.TokensOut,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role),
		Action: "ai.grade_suggest", TargetType: "question", TargetID: strconv.FormatInt(questionID, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"suggestion": resp.Content, "alias": resp.Alias, "model": resp.Model,
		"model_version": resp.ModelVersion, "filing_no": resp.FilingNo,
		"note": "AI 建议仅供教师参考，最终分数由教师提交并写审计", "at": time.Now().UTC().Format(time.RFC3339),
	}})
}
