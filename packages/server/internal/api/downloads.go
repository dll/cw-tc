package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/offline"
	"github.com/cw-tc/server/internal/sysconfig"
	"github.com/gin-gonic/gin"
)

// listDownloads GET /me/downloads —— 离线下载清单（有效期 / 状态，PRD §7.4 B-11）。
func (s *Server) listDownloads(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	items, err := offline.New(s.dbConn).List(c.Request.Context(), student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "下载清单查询失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items": items,
		"note":  "占用空间由客户端统计；服务端只记录授权与有效期",
	}})
}

// requestDownload POST /me/downloads —— 申请离线下载授权（已解锁 + 权益有效才签发）。
func (s *Server) requestDownload(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		CourseCode string `json:"course_code"`
		LectureNo  int    `json:"lecture_no"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.CourseCode == "" || input.LectureNo < 1 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	validDays := sysconfig.New(s.dbConn).Int(c.Request.Context(),
		sysconfig.KeyOfflineValidDays, sysconfig.DefaultOfflineValidDays)
	grant, err := offline.New(s.dbConn).Request(c.Request.Context(), student.ID, input.CourseCode, input.LectureNo, validDays)
	if err != nil {
		switch {
		case errors.Is(err, offline.ErrCourseNotFound), errors.Is(err, offline.ErrLectureNotFound):
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: err.Error()})
		case errors.Is(err, offline.ErrForbidden):
			c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: err.Error()})
		default:
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "签发下载授权失败"})
		}
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: grant})
}

// deleteDownload DELETE /me/downloads/:lectureNo —— 删除单讲下载（客户端同步清理本地文件）。
func (s *Server) deleteDownload(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	lectureNo, err := strconv.Atoi(c.Param("lectureNo"))
	if err != nil || lectureNo < 1 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "讲次参数非法"})
		return
	}
	service := offline.New(s.dbConn)
	if code := c.Query("course_code"); code != "" {
		err = service.Delete(c.Request.Context(), student.ID, code, lectureNo)
	} else {
		err = service.DeleteByLectureNo(c.Request.Context(), student.ID, lectureNo)
	}
	if err != nil {
		if errors.Is(err, offline.ErrNotFound) {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "下载记录不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "删除下载记录失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
