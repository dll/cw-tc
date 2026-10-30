package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

type myScore struct {
	ResultID    int64   `json:"result_id"`
	CourseCode  string  `json:"course_code"`
	CourseName  string  `json:"course_name"`
	Level       string  `json:"level"`
	AutoScore   float64 `json:"auto_score"`
	ManualScore float64 `json:"manual_score"`
	TotalScore  float64 `json:"total_score"`
	Band        string  `json:"band"`
	IsPass      bool    `json:"is_pass"`
	GradedAt    string  `json:"graded_at"`
	ExamAt      string  `json:"exam_at"`
	CertNo      string  `json:"cert_no"`
}

// listMyScores GET /me/scores —— 历次测评成绩（不覆盖、失败记录也展示）。
func (s *Server) listMyScores(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成绩服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT r.id, co.course_code, co.name, r.level, r.auto_score, r.manual_score, r.total_score,
		        COALESCE(r.band,''), r.is_pass, COALESCE(r.graded_at,''),
		        COALESCE(s.submitted_at, s.started_at, r.created_at),
		        COALESCE((SELECT c.cert_no FROM certificate c
		                  WHERE c.student_id=r.student_id AND c.course_id=r.course_id AND c.level=r.level
		                  ORDER BY c.issued_at DESC, c.id DESC LIMIT 1),'')
		 FROM exam_result r
		 JOIN course co ON co.id=r.course_id
		 LEFT JOIN exam_session s ON s.id=r.exam_session_id
		 WHERE r.student_id=$1
		 ORDER BY r.created_at DESC, r.id DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成绩查询失败"})
		return
	}
	defer rows.Close()
	items := make([]myScore, 0)
	for rows.Next() {
		var item myScore
		if err := rows.Scan(&item.ResultID, &item.CourseCode, &item.CourseName, &item.Level,
			&item.AutoScore, &item.ManualScore, &item.TotalScore, &item.Band, &item.IsPass,
			&item.GradedAt, &item.ExamAt, &item.CertNo); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成绩查询失败"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成绩查询失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
