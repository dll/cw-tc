package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/knowledge"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminRebuildKnowledge POST /admin/courses/:code/kb/rebuild —— 一键重建课程知识库。
func (s *Server) adminRebuildKnowledge(c *gin.Context) {
	if s.cfg == nil || strings.TrimSpace(s.cfg.ContentDir) == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "内容目录未配置，无法重建知识库"})
		return
	}
	stats, err := knowledge.BuildCourse(c.Request.Context(), s.dbConn, s.cfg.ContentDir, c.Param("code"))
	if err != nil {
		if errors.Is(err, knowledge.ErrCourseMissing) {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "知识库重建失败"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	after, _ := json.Marshal(stats)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.kb_rebuild", TargetType: "course", TargetID: stats.CourseCode,
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: stats})
}

// adminKnowledgeStatus GET /admin/kb/status —— 切片数 / 资源数 / 最后构建时间。
func (s *Server) adminKnowledgeStatus(c *gin.Context) {
	code := strings.TrimSpace(c.Query("course_code"))
	items, err := knowledge.ListStatus(c.Request.Context(), s.dbConn, code)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "知识库状态读取失败"})
		return
	}
	if code != "" && len(items) == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	totalChunks, totalAssets := 0, 0
	for _, item := range items {
		totalChunks += item.Chunks
		totalAssets += item.Assets
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items": items, "total_chunks": totalChunks, "total_assets": totalAssets,
	}})
}
