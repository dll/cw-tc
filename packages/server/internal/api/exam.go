package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/cw-tc/server/internal/course"
	"github.com/cw-tc/server/internal/exam"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// startExam POST /exams —— 组卷 + 开考，返回 session_id 与试卷（不含答案）。
func (s *Server) startExam(c *gin.Context) {
	var req struct {
		CourseID   int64  `json:"course_id"`
		CourseCode string `json:"course_code"`
		Level      string `json:"level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Level == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评服务不可用"})
		return
	}
	// 允许用公开标识 course_code 开考（客户端只有课程码，没有内部 id）
	if req.CourseID <= 0 && req.CourseCode != "" {
		var cid int64
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT id FROM course WHERE course_code = $1`, req.CourseCode).Scan(&cid); err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "课程不存在"})
			return
		}
		req.CourseID = cid
	}
	if req.CourseID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	ctx := c.Request.Context()
	// 紧急纠错处理中：暂停新测评与发证，但不影响已购学员继续学习。
	var courseCode string
	_ = s.dbConn.QueryRowContext(ctx, `SELECT course_code FROM course WHERE id = $1`, req.CourseID).Scan(&courseCode)
	if course.HasPendingUrgentFix(ctx, s.dbConn, courseCode) {
		c.JSON(http.StatusConflict, envelope{Code: 10803, Msg: "课程内容核对中，暂不能开始测评"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
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
		cn, issueErr := issueCertForSession(ctx, s.dbConn, sessionID, res, []byte(s.cfg.IDCardEncKey))
		if issueErr != nil {
			// 通过测评不等于已发证；缺少合法身份证时必须明确提示并阻止生成“空证书”结果。
			if errors.Is(issueErr, exam.ErrCertIdentityRequired) {
				c.JSON(http.StatusConflict, envelope{Code: 10507, Msg: issueErr.Error()})
				return
			}
			c.JSON(http.StatusConflict, envelope{Code: 10506, Msg: "证书签发失败"})
			return
		}
		certNo = cn
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	s.notifyExamResult(ctx, student.ID, res.IsPass, certNo)
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
func issueCertForSession(ctx context.Context, db *sql.DB, sessionID int64, res exam.Result, idCardEncKey []byte) (string, error) {
	var studentID, courseID int64
	var level string
	if err := db.QueryRowContext(ctx,
		`SELECT student_id, course_id, level FROM exam_session WHERE id = $1`, sessionID).
		Scan(&studentID, &courseID, &level); err != nil {
		return "", err
	}
	var courseCode string
	if err := db.QueryRowContext(ctx,
		`SELECT course_code FROM course WHERE id = $1`, courseID).Scan(&courseCode); err != nil {
		return "", err
	}
	return exam.IssueCertificate(ctx, db, exam.CertIssueInput{
		StudentID:    studentID,
		CourseID:     courseID,
		CourseCode:   courseCode,
		Level:        level,
		Score:        res.TotalScore,
		Band:         res.Band,
		IDCardEncKey: idCardEncKey,
	})
}

// resumeExam GET /exams/:sessionID —— 断线重连恢复（答案 + 剩余时间服务端计算）。
func (s *Server) resumeExam(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("sessionID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评服务不可用"})
		return
	}
	state, err := exam.Resume(c.Request.Context(), s.dbConn, sessionID, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10801, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: state})
}

// saveExamAnswer POST /exams/:sessionID/answers —— 按题即时持久化（断线不丢）。
func (s *Server) saveExamAnswer(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("sessionID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		QNo    string `json:"qno"`
		Answer string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.QNo == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评服务不可用"})
		return
	}
	if err := exam.SaveAnswer(c.Request.Context(), s.dbConn, sessionID, req.QNo, req.Answer); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10801, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
