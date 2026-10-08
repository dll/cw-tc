package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/cw-tc/server/internal/resource"
	"github.com/gin-gonic/gin"
)

// listLectureResources 返回某讲的资源清单（含短时签名 URL）。
func (s *Server) listLectureResources(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return
	}
	var courseID, lectureID int64
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT c.id, l.id FROM course c JOIN lecture l ON l.course_id = c.id WHERE c.course_code = $1 AND c.status = 'published' AND l.no = $2`,
		c.Param("courseCode"), no).Scan(&courseID, &lectureID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT kind, title, storage_key, size_bytes FROM asset WHERE lecture_id = $1 ORDER BY sort ASC, id ASC`, lectureID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	defer rows.Close()
	secret := []byte(s.cfg.SessionSecret)
	items := make([]gin.H, 0)
	for rows.Next() {
		var kind, title, storageKey string
		var sizeBytes sql.NullInt64
		if err := rows.Scan(&kind, &title, &storageKey, &sizeBytes); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
			return
		}
		expires := time.Now().Add(10 * time.Minute)
		sig := resource.Sign(secret, storageKey, expires)
		items = append(items, gin.H{"kind": kind, "title": title, "storage_key": storageKey, "size_bytes": sizeBytes.Int64, "expires_at": expires.Unix(), "signature": sig})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
