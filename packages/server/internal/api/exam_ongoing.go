package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/exam"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// ongoingExam GET /me/exams/ongoing —— 未完成测评恢复（断线 / 换设备）。
func (s *Server) ongoingExam(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	courseCode := c.Query("course_code")
	level := c.Query("level")
	if courseCode != "" && level != "" {
		var courseID int64
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT id FROM course WHERE course_code = $1`, courseCode).Scan(&courseID); err == nil {
			s.expireStaleExams(c.Request.Context(), student.ID, courseID, level)
		}
	}
	var sessionID int64
	var sessionLevel, paper, deadlineRaw string
	var answersRaw sql.NullString
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT s.id, s.level, s.paper, s.answers, s.server_deadline_at
		 FROM exam_session s JOIN course c ON c.id=s.course_id
		 WHERE s.student_id=$1 AND s.status='ongoing'
		   AND ($2='' OR c.course_code=$2) AND ($3='' OR s.level=$3)
		 ORDER BY s.id DESC LIMIT 1`, student.ID, courseCode, level).
		Scan(&sessionID, &sessionLevel, &paper, &answersRaw, &deadlineRaw)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"session": nil}})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评服务不可用"})
		return
	}
	deadline, err := time.Parse(time.RFC3339, deadlineRaw)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评状态异常"})
		return
	}
	remaining := int64(deadline.Sub(time.Now().UTC()).Seconds())
	var paperItems []json.RawMessage
	_ = json.Unmarshal([]byte(paper), &paperItems)
	answers := map[string]string{}
	if answersRaw.Valid {
		_ = json.Unmarshal([]byte(answersRaw.String), &answers)
	}
	deviceKey := deviceKeyFromRequest(c)
	activeKey := exam.DeviceKeyFromAnswers(answers)
	if remaining > 0 && activeKey != "" && deviceKey != "" && activeKey != deviceKey {
		c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
			"session": nil, "needs_takeover": true, "session_id": sessionID,
		}})
		return
	}
	if remaining > 0 && activeKey == "" && deviceKey != "" {
		// 历史会话没有设备标识：首个带设备标识的请求认领为当前设备。
		_ = exam.SetDeviceKey(c.Request.Context(), s.dbConn, sessionID, deviceKey)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"session_id": sessionID, "level": sessionLevel, "paper": paperItems,
		"answers": exam.WithoutDeviceKey(answers), "remaining_secs": remaining, "expired": remaining <= 0,
	}})
}
