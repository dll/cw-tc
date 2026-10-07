package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/exam"
	"github.com/gin-gonic/gin"
)

// verifyCert 证书验真 API（公开，四态；PRD §7.8 D-4 · API §2.6）。
func (s *Server) verifyCert(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "验真服务不可用"})
		return
	}
	result, err := exam.VerifyCert(c.Request.Context(), s.dbConn, c.Param("certNo"), time.Now())
	if errors.Is(err, exam.ErrCertNotFound) {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "证书不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "验真服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"cert_no":        result.CertNo,
		"state":          string(result.State),
		"course":         result.CourseName,
		"level":          result.Level,
		"name":           result.StudentName,
		"issued_at":      result.IssuedAt.Format(time.RFC3339),
		"expires_at":     result.ExpiresAt.Format(time.RFC3339),
		"revoked_reason": result.RevokedReason,
	}})
}
