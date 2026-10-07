package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/exam"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// startExam POST /exams —— 组卷 + 开考，返回 session_id 与试卷（不含答案）。
func (s *Server) startExam(c *gin.Context) {
	var req struct {
		CourseID int64  `json:"course_id"`
		Level    string `json:"level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CourseID <= 0 || req.Level == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	ctx := c.Request.Context()
	paper, err := exam.BuildPaper(ctx, s.dbConn, req.CourseID, req.Level, 0)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
		return
	}
	sessionID, err := exam.StartSession(ctx, s.dbConn, student.ID, req.CourseID, req.Level, paper)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10801, Msg: err.Error()})
		return
	}
	var paperJSON json.RawMessage
	_ = json.Unmarshal([]byte(paper), &paperJSON)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"session_id": sessionID, "paper": paperJSON}})
}

// submitExam POST /exams/:sessionID/submit —— 交卷判分，通过则自动发证。
func (s *Server) submitExam(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("sessionID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评服务不可用"})
		return
	}
	ctx := c.Request.Context()
	res, err := exam.SubmitExam(ctx, s.dbConn, sessionID, req.Answers)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10801, Msg: err.Error()})
		return
	}
	certNo := ""
	if res.IsPass {
		if cn, err := issueCertForSession(ctx, s.dbConn, sessionID, res); err == nil {
			certNo = cn
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"session_id":  res.SessionID,
		"auto_score":  res.AutoScore,
		"total_score": res.TotalScore,
		"is_pass":     res.IsPass,
		"band":        res.Band,
		"cert_no":     certNo,
	}})
}

// issueCertForSession 通过后发证：从会话取 student/course/level，再取 course_code。
func issueCertForSession(ctx context.Context, db *sql.DB, sessionID int64, res exam.Result) (string, error) {
	var studentID, courseID int64
	var level string
	if err := db.QueryRowContext(ctx,
		`SELECT student_id, course_id, level FROM exam_session WHERE id = ?`, sessionID).
		Scan(&studentID, &courseID, &level); err != nil {
		return "", err
	}
	var courseCode string
	if err := db.QueryRowContext(ctx,
		`SELECT course_code FROM course WHERE id = ?`, courseID).Scan(&courseCode); err != nil {
		return "", err
	}
	return exam.IssueCertificate(ctx, db, exam.CertIssueInput{
		StudentID:  studentID,
		CourseID:   courseID,
		CourseCode: courseCode,
		Level:      level,
		Score:      res.TotalScore,
		Band:       res.Band,
	})
}
