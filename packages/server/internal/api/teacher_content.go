package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/course"
	"github.com/cw-tc/server/internal/courseimport"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

const teacherUploadLimit = 512 << 20 // 512 MB

type teacherCourseInfo struct {
	ID      int64
	Source  string
	Status  string
	Teacher sql.NullInt64
}

func (s *Server) loadTeacherCourse(c *gin.Context, code string) (teacherCourseInfo, bool) {
	var info teacherCourseInfo
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, source, status, teacher_id FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).
		Scan(&info.ID, &info.Source, &info.Status, &info.Teacher); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
			return info, false
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程读取失败"})
		return info, false
	}
	st := c.MustGet(staffContextKey).(staff.Staff)
	if !info.Teacher.Valid || info.Teacher.Int64 != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "只能管理自己的课程"})
		return info, false
	}
	return info, true
}

func teacherEditable(info teacherCourseInfo) bool {
	return info.Source == "platform" && (info.Status == "proposed" || info.Status == "building" || info.Status == "rejected")
}

// teacherCourseContent GET /teacher/courses/:code/content —— 讲次与资源清单（只读自己的课）。
func (s *Server) teacherCourseContent(c *gin.Context) {
	info, ok := s.loadTeacherCourse(c, c.Param("code"))
	if !ok {
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, no, title, is_preview FROM lecture WHERE course_id = $1 ORDER BY no`, info.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容读取失败"})
		return
	}
	defer rows.Close()
	lectures := make([]gin.H, 0)
	lectureIDs := make([]int64, 0)
	indexByID := map[int64]int{}
	for rows.Next() {
		var id int64
		var no int
		var title string
		var preview bool
		if err := rows.Scan(&id, &no, &title, &preview); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容读取失败"})
			return
		}
		indexByID[id] = len(lectures)
		lectureIDs = append(lectureIDs, id)
		lectures = append(lectures, gin.H{
			"id": id, "no": no, "title": title, "is_preview": preview, "assets": []gin.H{},
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容读取失败"})
		return
	}
	if len(lectureIDs) > 0 {
		placeholders := make([]string, 0, len(lectureIDs))
		args := make([]any, 0, len(lectureIDs))
		for i, id := range lectureIDs {
			args = append(args, id)
			placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
		}
		assetRows, err := s.dbConn.QueryContext(c.Request.Context(),
			`SELECT id, lecture_id, kind, title, storage_key, COALESCE(size_bytes,0), sort, created_at
			 FROM asset WHERE lecture_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY sort, id`, args...)
		if err == nil {
			for assetRows.Next() {
				var id, lectureID int64
				var size int64
				var kind, title, storageKey, createdAt string
				var sortOrder int
				if assetRows.Scan(&id, &lectureID, &kind, &title, &storageKey, &size, &sortOrder, &createdAt) == nil {
					if idx, ok := indexByID[lectureID]; ok {
						items := lectures[idx]["assets"].([]gin.H)
						lectures[idx]["assets"] = append(items, gin.H{
							"id": id, "kind": kind, "title": title, "storage_key": storageKey,
							"size_bytes": size, "sort": sortOrder, "created_at": createdAt,
						})
					}
				}
			}
			assetRows.Close()
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"course_id": info.ID, "source": info.Source, "status": info.Status,
		"editable": teacherEditable(info), "lectures": lectures,
	}})
}

// teacherAddLecture POST /teacher/courses/:code/lectures —— 平台课程新增讲次。
func (s *Server) teacherAddLecture(c *gin.Context) {
	info, ok := s.loadTeacherCourse(c, c.Param("code"))
	if !ok {
		return
	}
	if !teacherEditable(info) {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "当前状态不可编辑内容"})
		return
	}
	var input struct {
		Title     string `json:"title"`
		IsPreview bool   `json:"is_preview"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Title) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "讲次标题必填"})
		return
	}
	st := c.MustGet(staffContextKey).(staff.Staff)
	id, err := course.AddLecture(c.Request.Context(), s.dbConn, course.LectureInput{
		CourseID: info.ID, TeacherID: st.ID, Title: strings.TrimSpace(input.Title), IsPreview: input.IsPreview,
	})
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: err.Error()})
		return
	}
	s.touchTeacherCourse(c, info.ID)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

// teacherUploadAsset POST /teacher/courses/:code/lectures/:no/assets —— 上传单个资源。
func (s *Server) teacherUploadAsset(c *gin.Context) {
	info, ok := s.loadTeacherCourse(c, c.Param("code"))
	if !ok {
		return
	}
	if !teacherEditable(info) {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "当前状态不可编辑内容"})
		return
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil || no < 1 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "讲次号非法"})
		return
	}
	var lectureID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id FROM lecture WHERE course_id = $1 AND no = $2`, info.ID, no).Scan(&lectureID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return
	}
	if s.cfg == nil || s.cfg.ContentDir == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容目录未配置"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, teacherUploadLimit)
	if err := c.Request.ParseMultipartForm(16 << 20); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "上传内容过大或格式错误"})
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "请选择资源文件"})
		return
	}
	filename := path.Base(strings.ReplaceAll(fileHeader.Filename, "\\", "/"))
	if filename == "." || filename == "/" || filename == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "文件名非法"})
		return
	}
	kind, err := courseimport.DetectKind(filename, c.PostForm("kind"))
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "不支持的资源类型"})
		return
	}
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = filename
	}
	storageKey := courseimport.StorageKey(c.Param("code"), no, kind, filename)
	var existing int
	_ = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM asset WHERE storage_key = $1`, storageKey).Scan(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "同名资源已存在，请改名后重试"})
		return
	}

	temp, err := os.CreateTemp("", "cwtc-upload-*")
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
		return
	}
	defer os.Remove(temp.Name())
	file, err := fileHeader.Open()
	if err != nil {
		temp.Close()
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "上传失败"})
		return
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(temp, hash), file)
	_ = file.Close()
	_ = temp.Close()
	if copyErr != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
		return
	}
	digest := hex.EncodeToString(hash.Sum(nil))

	localPath := filepath.Join(s.cfg.ContentDir, filepath.FromSlash(storageKey))
	if (kind == "video" || kind == "audio") && s.cosStore != nil && s.cosStore.Enabled() && !s.cfg.MediaForceFile {
		source, err := os.Open(temp.Name())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
			return
		}
		uploadErr := s.cosStore.Upload(c.Request.Context(), storageKey, source)
		_ = source.Close()
		if uploadErr != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传到对象存储失败"})
			return
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
			return
		}
		source, err := os.Open(temp.Name())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
			return
		}
		target, err := os.Create(localPath)
		if err != nil {
			_ = source.Close()
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
			return
		}
		_, copyErr = io.Copy(target, source)
		_ = target.Close()
		_ = source.Close()
		if copyErr != nil {
			_ = os.Remove(localPath)
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "上传失败"})
			return
		}
	}

	var assetID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO asset(lecture_id, kind, title, storage_key, size_bytes, sha256, sort)
		 VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		lectureID, kind, title, storageKey, size, digest, 0).Scan(&assetID); err != nil {
		_ = os.Remove(localPath)
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "资源写入失败"})
		return
	}
	s.touchTeacherCourse(c, info.ID)
	st := c.MustGet(staffContextKey).(staff.Staff)
	after, _ := json.Marshal(gin.H{"kind": kind, "storage_key": storageKey, "size": size})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role),
		Action: "teacher.asset_upload", TargetType: "asset", TargetID: strconv.FormatInt(assetID, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"id": assetID, "kind": kind, "title": title, "storage_key": storageKey,
		"size_bytes": size, "sha256": digest,
	}})
}

// teacherDeleteAsset DELETE /teacher/courses/:code/lectures/:no/assets/:assetID —— 删除自己的资源。
func (s *Server) teacherDeleteAsset(c *gin.Context) {
	info, ok := s.loadTeacherCourse(c, c.Param("code"))
	if !ok {
		return
	}
	if !teacherEditable(info) {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "当前状态不可编辑内容"})
		return
	}
	assetID, err := strconv.ParseInt(c.Param("assetID"), 10, 64)
	if err != nil || assetID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "资源编号非法"})
		return
	}
	var storageKey string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT a.storage_key FROM asset a JOIN lecture l ON l.id = a.lecture_id
		 WHERE a.id = $1 AND l.course_id = $2`, assetID, info.ID).Scan(&storageKey); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "资源不存在"})
		return
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(), `DELETE FROM asset WHERE id = $1`, assetID); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "资源删除失败"})
		return
	}
	if s.cfg != nil && s.cfg.ContentDir != "" {
		_ = os.Remove(filepath.Join(s.cfg.ContentDir, filepath.FromSlash(storageKey)))
	}
	s.touchTeacherCourse(c, info.ID)
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) touchTeacherCourse(c *gin.Context, courseID int64) {
	_, _ = s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course SET lecture_count = (SELECT COUNT(*) FROM lecture WHERE course_id = $1),
		        content_version = content_version + 1, updated_at = $2
		 WHERE id = $1`, courseID, time.Now().UTC().Format(time.RFC3339))
}
