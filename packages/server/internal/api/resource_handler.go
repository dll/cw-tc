package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"os"
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
	var freePreviewLectures int
	var status string
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT c.id, l.id, l.is_preview, c.free_preview_lectures, c.status FROM course c JOIN lecture l ON l.course_id = c.id WHERE c.course_code = $1 AND c.deleted_at IS NULL AND l.no = $2`,
		c.Param("courseCode"), no).Scan(&courseID, &lectureID, &isPreview, &freePreviewLectures, &status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	// 免费讲次 = 上游试学标记 OR 讲次号 <= 平台配置的免费预览讲次数。
	// 下架课程不再对外提供试学；已购学员仍可继续访问（PRD B-8）。
	isPreview = status == "published" && (isPreview || no <= freePreviewLectures)
	student := c.MustGet(studentContextKey).(identity.Student)
	if status != "published" {
		if !s.hasActiveEntitlement(c.Request.Context(), student.ID, courseID) {
			c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
			return
		}
	} else if !s.checkResourceAccess(c, student.ID, courseID, isPreview) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT id, kind, title, storage_key, size_bytes FROM asset WHERE lecture_id = $1 ORDER BY CASE WHEN kind = 'video' THEN 0 ELSE 1 END, sort ASC, id ASC`, lectureID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	defer rows.Close()
	secret := []byte(s.cfg.SessionSecret)
	items := make([]gin.H, 0)
	for rows.Next() {
		var assetID int64
		var kind, title, storageKey string
		var sizeBytes sql.NullInt64
		if err := rows.Scan(&assetID, &kind, &title, &storageKey, &sizeBytes); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
			return
		}
		expires := time.Now().Add(10 * time.Minute)
		sig := resource.Sign(secret, storageKey, expires)
		item := gin.H{"kind": kind, "title": title, "storage_key": storageKey, "size_bytes": sizeBytes.Int64, "expires_at": expires.Unix(), "signature": sig}
		if kind == "html" {
			inlineExpires := time.Now().Add(2 * time.Hour)
			item["inline_base_url"] = "/api/v1/assets/inline/" + strconv.FormatInt(student.ID, 10) + "/" + strconv.FormatInt(assetID, 10) + "/" + strconv.FormatInt(inlineExpires.Unix(), 10) + "/" + resource.Sign(secret, inlineSignatureKey(assetID, student.ID), inlineExpires) + "/"
		}
		// 视频/音频：COS 可读则下发预签名 URL（Web 播放器无法带 Bearer 头，需自认证 URL）。
		// COS 不可用时下发绑定学员的短时签名 URL；播放器不能可靠地附加 Bearer/Cookie。
		if isCosBackedKey(storageKey) {
			if s.cosAvailable(c.Request.Context(), storageKey) {
				if u, err := s.cosStore.PresignGet(c.Request.Context(), storageKey, 2*time.Hour); err == nil {
					item["url"] = u
				}
			}
			if _, ok := item["url"]; !ok {
				mediaExpires := time.Now().Add(2 * time.Hour)
				q := url.Values{}
				q.Set("key", storageKey)
				q.Set("expires", strconv.FormatInt(mediaExpires.Unix(), 10))
				q.Set("student", strconv.FormatInt(student.ID, 10))
				q.Set("sig", resource.Sign(secret, mediaSignatureKey(storageKey, student.ID), mediaExpires))
				item["url"] = "/api/v1/assets/media?" + q.Encode()
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
	var lectureNo, previewCount int
	var status string
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT a.kind, l.course_id, l.no, l.is_preview, co.free_preview_lectures, co.status FROM asset a JOIN lecture l ON l.id = a.lecture_id JOIN course co ON co.id = l.course_id WHERE a.storage_key = $1 AND co.deleted_at IS NULL`, key).Scan(&kind, &courseID, &lectureNo, &isPreview, &previewCount, &status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if status != "published" {
		if !s.hasActiveEntitlement(c.Request.Context(), student.ID, courseID) {
			c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
			return
		}
	} else if !s.checkResourceAccess(c, student.ID, courseID, isPreview || lectureNo <= previewCount) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	s.serveAsset(c, key, kind)
}

// streamSignedMedia 使用绑定学员与资源的短时签名，供浏览器原生 video/audio 请求使用。
// 每次请求重新检查权益，退款或撤销权益后即使签名未过期也不能继续读取。
func (s *Server) streamSignedMedia(c *gin.Context) {
	if s.cfg == nil || s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "资源不可用"})
		return
	}
	key := c.Query("key")
	studentID, studentErr := strconv.ParseInt(c.Query("student"), 10, 64)
	expiresUnix, expiresErr := strconv.ParseInt(c.Query("expires"), 10, 64)
	if key == "" || studentErr != nil || studentID <= 0 || expiresErr != nil || !isCosBackedKey(key) ||
		!resource.Verify([]byte(s.cfg.SessionSecret), mediaSignatureKey(key, studentID), time.Unix(expiresUnix, 0), c.Query("sig")) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "签名无效或已过期"})
		return
	}
	var kind string
	var courseID int64
	var lectureNo, previewCount int
	var isPreview bool
	var status string
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT a.kind, l.course_id, l.no, l.is_preview, co.free_preview_lectures, co.status
		 FROM asset a JOIN lecture l ON l.id = a.lecture_id JOIN course co ON co.id = l.course_id
		 JOIN student st ON st.id = $2 AND st.status = 'active'
		 WHERE a.storage_key = $1 AND co.deleted_at IS NULL`, key, studentID,
	).Scan(&kind, &courseID, &lectureNo, &isPreview, &previewCount, &status)
	if err != nil || (kind != "video" && kind != "audio") {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不存在"})
		return
	}
	if status != "published" {
		if !s.hasActiveEntitlement(c.Request.Context(), studentID, courseID) {
			c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
			return
		}
	} else if !s.checkResourceAccess(c, studentID, courseID, isPreview || lectureNo <= previewCount) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	s.serveAsset(c, key, kind)
}

func mediaSignatureKey(key string, studentID int64) string {
	return "media:" + strconv.FormatInt(studentID, 10) + ":" + key
}

func inlineSignatureKey(assetID, studentID int64) string {
	return "inline:" + strconv.FormatInt(studentID, 10) + ":" + strconv.FormatInt(assetID, 10)
}

// streamInlineAsset 为 HTML 讲义中的相对资源提供受控基址。
// 签名只授权同一讲、同一目录下的课程资源，每次请求仍检查账号和学习权益。
func (s *Server) streamInlineAsset(c *gin.Context) {
	if s.cfg == nil || s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "资源不可用"})
		return
	}
	studentID, studentErr := strconv.ParseInt(c.Param("student"), 10, 64)
	assetID, assetErr := strconv.ParseInt(c.Param("asset"), 10, 64)
	expiresUnix, expiresErr := strconv.ParseInt(c.Param("expires"), 10, 64)
	if studentErr != nil || studentID <= 0 || assetErr != nil || assetID <= 0 || expiresErr != nil ||
		!resource.Verify([]byte(s.cfg.SessionSecret), inlineSignatureKey(assetID, studentID), time.Unix(expiresUnix, 0), c.Param("sig")) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "签名无效或已过期"})
		return
	}
	var htmlKey string
	var lectureID, courseID int64
	var lectureNo, previewCount int
	var isPreview bool
	var status string
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT a.storage_key, l.id, l.course_id, l.no, l.is_preview, co.free_preview_lectures, co.status
		 FROM asset a JOIN lecture l ON l.id = a.lecture_id JOIN course co ON co.id = l.course_id
		 JOIN student st ON st.id = $2 AND st.status = 'active'
		 WHERE a.id = $1 AND a.kind = 'html' AND co.deleted_at IS NULL`, assetID, studentID,
	).Scan(&htmlKey, &lectureID, &courseID, &lectureNo, &isPreview, &previewCount, &status)
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不存在"})
		return
	}
	if status != "published" {
		if !s.hasActiveEntitlement(c.Request.Context(), studentID, courseID) {
			c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
			return
		}
	} else if !s.checkResourceAccess(c, studentID, courseID, isPreview || lectureNo <= previewCount) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	relative := strings.TrimPrefix(c.Param("relative"), "/")
	base := filepath.ToSlash(filepath.Dir(htmlKey))
	key := filepath.ToSlash(filepath.Clean(filepath.Join(base, filepath.FromSlash(relative))))
	if relative == "" || key == base || !strings.HasPrefix(key, base+"/") {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "非法路径"})
		return
	}
	var kind string
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT kind FROM asset WHERE lecture_id = $1 AND storage_key = $2`, lectureID, key).Scan(&kind)
	if err == sql.ErrNoRows && strings.HasPrefix(key, base+"/assets/") {
		// 上游同步仅登记教学资源类型，HTML 内的配图没有 asset 行。
		// 图片仍限制在该 HTML 的 assets 目录和当前学员的授权范围内。
		if s.cfg.ContentDir != "" {
			imagePath := filepath.Join(s.cfg.ContentDir, filepath.FromSlash(key))
			assetDir := filepath.Join(s.cfg.ContentDir, filepath.FromSlash(base), "assets")
			resolvedImage, imageErr := filepath.EvalSymlinks(imagePath)
			resolvedDir, dirErr := filepath.EvalSymlinks(assetDir)
			if imageErr == nil && dirErr == nil && strings.HasPrefix(resolvedImage, resolvedDir+string(os.PathSeparator)) {
				switch strings.ToLower(filepath.Ext(key)) {
				case ".jpg", ".jpeg", ".png", ".gif", ".webp":
					kind = "image"
					err = nil
				}
			}
		}
	}
	if err != nil || (kind != "audio" && kind != "slide" && kind != "html" && kind != "image") {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不存在"})
		return
	}
	s.serveAsset(c, key, kind)
}

// serveAsset 下发已完成签名与权限校验的资源。
func (s *Server) serveAsset(c *gin.Context, key, kind string) {
	if s.cfg.ContentDir == "" && (!isCosBackedKey(key) || !s.cosAvailable(c.Request.Context(), key)) {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源文件不可用"})
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
	// COS 欠费/停服（cosAvailable=false）或 MediaForceFile 时，跳过 COS 直接读文件服务。
	if isCosBackedKey(key) && s.cosAvailable(c.Request.Context(), key) {
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

// checkResourceAccess 校验学员对课程资源有访问权（免费课 / 有效权益，或试学讲次）。
func (s *Server) checkResourceAccess(c *gin.Context, studentID, courseID int64, isPreview bool) bool {
	if isPreview {
		return true // 试学讲次免费
	}
	// 免费课（price_cents=0）直接放行，无需购买。
	var priceCents int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT price_cents FROM course WHERE id = $1`, courseID).Scan(&priceCents); err == nil && priceCents == 0 {
		return true
	}
	return s.hasActiveEntitlement(c.Request.Context(), studentID, courseID)
}

// hasActiveEntitlement 判断学员是否已购/被授权该课程（未撤销即有效）。
func (s *Server) hasActiveEntitlement(ctx context.Context, studentID, courseID int64) bool {
	var n int
	if err := s.dbConn.QueryRowContext(ctx,
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

// cosAvailable 探测 COS 是否可读（带 5 分钟缓存）。
// COS 未配置、MediaForceFile 强制回退、或探测到欠费/停服（非 206）时返回 false，
// 调用方据此把视频/音频回退到文件服务（ContentDir）。
func (s *Server) cosAvailable(ctx context.Context, key string) bool {
	if s.cfg == nil || s.cfg.MediaForceFile || s.cosStore == nil || !s.cosStore.Enabled() {
		return false
	}
	s.cosProbeMu.Lock()
	defer s.cosProbeMu.Unlock()
	if cached, ok := s.cosProbes[key]; ok && time.Since(cached.at) < 5*time.Minute {
		return cached.ok
	}
	ok := s.cosStore.Probe(ctx, key)
	if s.cosProbes == nil {
		s.cosProbes = make(map[string]cosProbeResult)
	}
	s.cosProbes[key] = cosProbeResult{at: time.Now(), ok: ok}
	return ok
}
