package api

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/identity"
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
	var isPreview bool
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT c.id, l.id, l.is_preview FROM course c JOIN lecture l ON l.course_id = c.id WHERE c.course_code = $1 AND c.status = 'published' AND l.no = $2`,
		c.Param("courseCode"), no).Scan(&courseID, &lectureID, &isPreview)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if !s.checkResourceAccess(c, student.ID, courseID, isPreview) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT kind, title, storage_key, size_bytes FROM asset WHERE lecture_id = $1 ORDER BY CASE WHEN kind = 'video' THEN 0 ELSE 1 END, sort ASC, id ASC`, lectureID)
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
		item := gin.H{"kind": kind, "title": title, "storage_key": storageKey, "size_bytes": sizeBytes.Int64, "expires_at": expires.Unix(), "signature": sig}
		// 视频/音频直接下发 COS 预签名 URL（Web 播放器无法带 Bearer 头，需自认证 URL）。
		if s.cosStore != nil && s.cosStore.Enabled() && isCosBackedKey(storageKey) {
			if u, err := s.cosStore.PresignGet(c.Request.Context(), storageKey, 2*time.Hour); err == nil {
				item["url"] = u
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// downloadAsset GET /api/v1/assets/raw?key=&expires=&sig= —— 校验签名后返回资源文件。
// 只从 ContentDir 内取文件，防目录穿越；签名过期或不符一律 403。
func (s *Server) downloadAsset(c *gin.Context) {
	if s.cfg == nil || s.cfg.ContentDir == "" {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不可用"})
		return
	}
	key := c.Query("key")
	expiresUnix, err := strconv.ParseInt(c.Query("expires"), 10, 64)
	sig := c.Query("sig")
	if key == "" || sig == "" || err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	expires := time.Unix(expiresUnix, 0)
	if !resource.Verify([]byte(s.cfg.SessionSecret), key, expires, sig) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "签名无效或已过期"})
		return
	}
	// 防盗链：按 storage_key 反查所属课程与是否试学，校验学习权限。
	if s.dbConn == nil {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	var kind string
	var courseID int64
	var isPreview bool
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT a.kind, l.course_id, l.is_preview FROM asset a JOIN lecture l ON l.id = a.lecture_id WHERE a.storage_key = $1`, key).Scan(&kind, &courseID, &isPreview)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if !s.checkResourceAccess(c, student.ID, courseID, isPreview) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	// 源码可下载，其余仅查看/播放（防盗：非源码不给 attachment 下载头）。
	if kind == "code" {
		c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(key)+"\"")
	} else {
		c.Header("Content-Disposition", "inline")
	}
	// 视频/音频对象在 COS：预签名后 302 重定向（私有桶不能直接 GET，走签名 URL）。
	// 预签名失败回退文件服务，绝不在签名校验后 500。
	if s.cosStore != nil && s.cosStore.Enabled() && isCosBackedKey(key) {
		if u, err := s.cosStore.PresignGet(c.Request.Context(), key, 10*time.Minute); err == nil {
			c.Redirect(http.StatusFound, u)
			return
		}
	}
	base := filepath.Clean(s.cfg.ContentDir)
	filePath := filepath.Join(base, filepath.FromSlash(key))
	if !strings.HasPrefix(filepath.Clean(filePath), base+string(filepath.Separator)) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "非法路径"})
		return
	}
	c.File(filePath)
}

// checkResourceAccess 校验学员对课程资源有访问权（有效权益，或试学讲次）。
func (s *Server) checkResourceAccess(c *gin.Context, studentID, courseID int64, isPreview bool) bool {
	if isPreview {
		return true // 试学讲次免费
	}
	var n int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM entitlement WHERE student_id = $1 AND course_id = $2 AND revoked_at IS NULL`,
		studentID, courseID).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// isCosBackedKey 判断 storage_key 是否属于 COS 承载的大二进制资源（视频/音频）。
func isCosBackedKey(key string) bool {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".mp4", ".mov", ".m4v", ".mp3", ".wav", ".m4a", ".flac", ".aac":
		return true
	default:
		return false
	}
}
