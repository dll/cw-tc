package api

import (
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// listMyCerts GET /me/certs —— 学员自己的证书列表（含课程码/名称）。
func (s *Server) listMyCerts(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.cert_no, co.course_code, co.name, c.level, c.score, c.band, c.status, c.issued_at, c.expires_at
		 FROM certificate c JOIN course co ON c.course_id = co.id
		 WHERE c.student_id = ? ORDER BY c.issued_at DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var certNo, courseCode, courseName, level, band, status, issuedAt, expiresAt string
		var score float64
		if err := rows.Scan(&certNo, &courseCode, &courseName, &level, &score, &band, &status, &issuedAt, &expiresAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
			return
		}
		items = append(items, gin.H{
			"cert_no": certNo, "course_code": courseCode, "course_name": courseName,
			"level": level, "score": score, "band": band, "status": status,
			"issued_at": issuedAt, "expires_at": expiresAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
