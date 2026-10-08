package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/exam"
	"github.com/cw-tc/server/internal/staff"
)

// teacherGradeList GET /teacher/grade —— 教师待阅卷清单（仅自有课程，铁律 8）。
func (s *Server) teacherGradeList(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	items, err := exam.ListManualReviews(c.Request.Context(), s.dbConn, st.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// teacherGradeDetail GET /teacher/grade/:resultID —— 逐题详情（题干 + 学员作答 + 可判标记）。
func (s *Server) teacherGradeDetail(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	resultID, err := strconv.ParseInt(c.Param("resultID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	// 归属校验：该结果所属课程必须归本人（先查归属，再返回，铁律 8）
	var courseID, sessionID int64
	var level string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT r.course_id, r.exam_session_id, r.level FROM exam_result r WHERE r.id = ?`, resultID).
		Scan(&courseID, &sessionID, &level); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "结果不存在"})
		return
	}
	var owner int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT teacher_id FROM course WHERE id = ?`, courseID).Scan(&owner); err != nil || owner != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权评阅该课程"})
		return
	}
	var paperRaw, answersRaw string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT paper, COALESCE(answers, '{}') FROM exam_session WHERE id = ?`, sessionID).
		Scan(&paperRaw, &answersRaw); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "会话不存在"})
		return
	}
	var paper []struct {
		Qno   string  `json:"qno"`
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(paperRaw), &paper); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "试卷解析失败"})
		return
	}
	var answers map[string]string
	_ = json.Unmarshal([]byte(answersRaw), &answers)

	// 逐题取题干、可判标记与内部 id（评阅按内部 id 提交）。
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, qno, stem, gradability FROM question WHERE course_id = ? AND level = ?`, courseID, level)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	type qd struct {
		id         int64
		stem, grad string
	}
	qmap := map[string]qd{}
	for rows.Next() {
		var q qd
		var qno string
		if err := rows.Scan(&q.id, &qno, &q.stem, &q.grad); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		qmap[qno] = q
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}

	items := make([]gin.H, 0)
	for _, it := range paper {
		q := qmap[it.Qno]
		items = append(items, gin.H{
			"qno": it.Qno, "question_id": q.id, "stem": q.stem, "score": it.Score,
			"gradability": q.grad, "answer": answers[it.Qno],
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"result_id": resultID, "items": items}})
}

// teacherGradeSubmit POST /teacher/grade/:resultID/questions/:questionID —— 评阅一道主观题。
func (s *Server) teacherGradeSubmit(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	resultID, err1 := strconv.ParseInt(c.Param("resultID"), 10, 64)
	questionID, err2 := strconv.ParseInt(c.Param("questionID"), 10, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Score   float64 `json:"score"`
		Comment string  `json:"comment"`
		Points  string  `json:"points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Score < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	// 归属校验。
	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT course_id FROM exam_result WHERE id = ?`, resultID).Scan(&courseID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "结果不存在"})
		return
	}
	var owner int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT teacher_id FROM course WHERE id = ?`, courseID).Scan(&owner); err != nil || owner != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权评阅该课程"})
		return
	}
	if err := exam.GradeManual(c.Request.Context(), s.dbConn, resultID, questionID, st.ID, req.Score, req.Comment, req.Points); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
