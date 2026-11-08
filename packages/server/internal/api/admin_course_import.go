package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/courseimport"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// coursePackageLimit 限制上传 ZIP 的大小（上传层；解压后另有总量上限）。
const coursePackageLimit = 1 << 30 // 1 GB

// adminImportCoursePackage POST /admin/courses/import-package —— 上传 ZIP 课程资源包并导入。
//
// 资源包约定：根目录（或唯一一层顶层目录）含 manifest.json，资源路径相对清单所在目录。
// 服务端会先安全解压、校验清单与资源，再写入平台课程草稿（building）；不会自动上架。
func (s *Server) adminImportCoursePackage(c *gin.Context) {
	if s.dbConn == nil || s.cfg == nil || strings.TrimSpace(s.cfg.ContentDir) == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容目录未配置，无法导入"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, coursePackageLimit)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "资源包过大或上传格式不正确"})
		return
	}
	header, err := c.FormFile("file")
	if err != nil || header.Size <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "请选择 ZIP 课程资源包"})
		return
	}
	if !strings.EqualFold(filepath.Ext(header.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "仅支持 .zip 格式的课程资源包"})
		return
	}

	tempDir, err := os.MkdirTemp("", "cwtc-course-package-*")
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "导入失败：无法创建临时目录"})
		return
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "package.zip")
	size, digest, err := saveUploadedFile(header, zipPath)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "导入失败：资源包保存失败"})
		return
	}
	packageRoot, err := courseimport.ExtractPackage(zipPath, filepath.Join(tempDir, "extract"))
	if err != nil {
		writeCourseImportError(c, err)
		return
	}
	manifest, err := courseimport.ReadManifest(packageRoot)
	if err != nil {
		writeCourseImportError(c, err)
		return
	}
	// 管理员可在界面上指定归属教师，覆盖清单中的 teacher_id（避免手工查 id）。
	if raw := strings.TrimSpace(c.PostForm("teacher_id")); raw != "" {
		teacherID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || teacherID <= 0 {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "归属教师参数不合法"})
			return
		}
		manifest.TeacherID = &teacherID
	}
	// 上传包的资源路径相对 manifest.json 所在目录，不再依赖服务器上的 imports/ 预置目录。
	manifest.SourceRoot = "."

	importer := &courseimport.Importer{
		DB:         s.dbConn,
		ContentDir: s.cfg.ContentDir,
		ImportRoot: packageRoot,
	}
	if s.cosStore != nil && s.cosStore.Enabled() && !s.cfg.MediaForceFile {
		importer.Uploader = s.cosStore
	}
	result, err := importer.Import(c.Request.Context(), manifest)
	if err != nil {
		writeCourseImportError(c, err)
		return
	}

	admin := c.MustGet(staffContextKey).(staff.Staff)
	after, _ := json.Marshal(gin.H{
		"course_code": result.CourseCode, "name": manifest.Name,
		"lectures": result.Lectures, "assets": result.Assets,
		"package": header.Filename, "package_sha256": digest, "package_size": size,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.import", TargetType: "course", TargetID: result.CourseCode,
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"course_code": result.CourseCode, "name": manifest.Name,
		"lectures": result.Lectures, "assets": result.Assets,
		"storage_keys": result.Storage, "status": "building",
		"package": header.Filename, "package_sha256": digest,
		"note": "已导入为平台课程草稿；请核对内容后走审核上架流程",
	}})
}

// adminImportCourse POST /admin/courses/import —— 旧版 JSON 清单导入（需预先上传到服务器 imports/）。
// 保留用于脚本与兼容；管理后台使用 adminImportCoursePackage。
func (s *Server) adminImportCourse(c *gin.Context) {
	var input struct {
		Confirm  bool                  `json:"confirm"`
		Manifest courseimport.Manifest `json:"manifest"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "导入课程必须提交清单并二次确认"})
		return
	}
	if s.cfg == nil || s.cfg.ContentDir == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容目录未配置，无法导入"})
		return
	}
	importer := &courseimport.Importer{
		DB:         s.dbConn,
		ContentDir: s.cfg.ContentDir,
		ImportRoot: filepath.Join(s.cfg.ContentDir, "imports"),
	}
	if s.cosStore != nil && s.cosStore.Enabled() && !s.cfg.MediaForceFile {
		importer.Uploader = s.cosStore
	}
	result, err := importer.Import(c.Request.Context(), input.Manifest)
	if err != nil {
		writeCourseImportError(c, err)
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	after, _ := json.Marshal(gin.H{
		"course_code": result.CourseCode, "lectures": result.Lectures,
		"assets": result.Assets, "source_root": input.Manifest.SourceRoot,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.import", TargetType: "course", TargetID: result.CourseCode,
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"course_code": result.CourseCode, "lectures": result.Lectures,
		"assets": result.Assets, "storage_keys": result.Storage,
		"status": "building",
		"note":   "已导入为平台课程草稿；请核对内容后走审核上架流程",
	}})
}

func writeCourseImportError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, courseimport.ErrCourseExists):
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: err.Error()})
	case errors.Is(err, courseimport.ErrPackage),
		errors.Is(err, courseimport.ErrUnsupported),
		errors.Is(err, courseimport.ErrInput),
		errors.Is(err, courseimport.ErrSourceNotFound):
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
	default:
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程导入失败"})
	}
}

func saveUploadedFile(header *multipart.FileHeader, target string) (int64, string, error) {
	source, err := header.Open()
	if err != nil {
		return 0, "", err
	}
	defer source.Close()
	destination, err := os.Create(target)
	if err != nil {
		return 0, "", err
	}
	defer destination.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(destination, hash), io.LimitReader(source, coursePackageLimit+1))
	if err != nil {
		return 0, "", err
	}
	if size > coursePackageLimit {
		return 0, "", errors.New("资源包过大")
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}
