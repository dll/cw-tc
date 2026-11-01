package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/courseimport"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminImportCourse POST /admin/courses/import —— 从资源包导入新课程（平台课程草稿）。
//
// 资源包放在 `ContentDir/imports/<source_root>/`，清单只写相对路径；
// HTML 目录整目录复制，MP4/音频在配置 COS 时上传 COS，否则回落到本地。
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
		switch {
		case errors.Is(err, courseimport.ErrCourseExists):
			c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: err.Error()})
		case errors.Is(err, courseimport.ErrUnsupported), errors.Is(err, courseimport.ErrInput):
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
		case errors.Is(err, courseimport.ErrSourceNotFound):
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
		default:
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程导入失败"})
		}
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
