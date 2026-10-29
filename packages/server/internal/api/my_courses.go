package api

import (
	"database/sql"
	"net/http"
	"sort"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// listMyCourses GET /me/courses —— 我的课程（已解锁），带每讲学习进度。
func (s *Server) listMyCourses(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.id, c.course_code, c.name, c.intro, c.lecture_count, m.name
		 FROM entitlement e JOIN course c ON c.id = e.course_id
		 LEFT JOIN major m ON m.id = c.major_id
		 WHERE e.student_id = $1 AND e.revoked_at IS NULL AND c.deleted_at IS NULL`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	defer rows.Close()
	type courseRow struct {
		id           int64
		code, name   string
		intro        sql.NullString
		majorName    sql.NullString
		lectureCount int
	}
	courses := make([]courseRow, 0)
	for rows.Next() {
		var row courseRow
		if err := rows.Scan(&row.id, &row.code, &row.name, &row.intro, &row.lectureCount, &row.majorName); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
			return
		}
		courses = append(courses, row)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	// SQLite 单连接下不能在 rows 未关闭时再发查询；先读完课程再逐门统计。
	_ = rows.Close()

	items := make([]gin.H, 0, len(courses))
	for _, course := range courses {
		var total, completed int
		_ = s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT COUNT(*) FROM lecture WHERE course_id = $1`, course.id).Scan(&total)
		_ = s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT COUNT(*) FROM lecture_progress lp JOIN lecture l ON l.id = lp.lecture_id
			 WHERE lp.student_id = $1 AND l.course_id = $2 AND lp.completed_at IS NOT NULL`,
			student.ID, course.id).Scan(&completed)
		var lastLectureNo int
		var lastStudied sql.NullString
		_ = s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT l.no, lp.last_studied_at FROM lecture_progress lp JOIN lecture l ON l.id = lp.lecture_id
			 WHERE lp.student_id = $1 AND l.course_id = $2 ORDER BY lp.last_studied_at DESC LIMIT 1`,
			student.ID, course.id).Scan(&lastLectureNo, &lastStudied)
		percent := 0
		if total > 0 {
			percent = completed * 100 / total
		}
		status := "learning"
		if total > 0 && completed >= total {
			status = "completed"
		}
		if lastLectureNo < 1 {
			lastLectureNo = 1
		}
		items = append(items, gin.H{
			"course_code": course.code, "name": course.name, "intro": nullableString(course.intro),
			"major_name": nullableString(course.majorName), "lecture_count": total,
			"completed_lectures": completed, "percent": percent,
			"last_lecture_no": lastLectureNo, "last_studied_at": nullableString(lastStudied),
			"status": status, "unlocked": true,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, _ := items[i]["last_studied_at"].(string)
		right, _ := items[j]["last_studied_at"].(string)
		return left > right
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "total": len(items)}})
}

// reportCourseProgress PUT /me/courses/:code/progress —— 上报断点 / 标记本讲已学。
func (s *Server) reportCourseProgress(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	var req struct {
		LectureNo      int  `json:"lecture_no"`
		PositionSecond int  `json:"position_seconds"`
		Completed      bool `json:"completed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LectureNo < 1 || req.PositionSecond < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var courseID int64
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT c.id FROM course c JOIN entitlement e ON e.course_id = c.id
		 WHERE c.course_code = $1 AND c.deleted_at IS NULL AND e.student_id = $2 AND e.revoked_at IS NULL`,
		c.Param("code"), student.ID).Scan(&courseID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	var lectureID int64
	err = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id FROM lecture WHERE course_id = $1 AND no = $2`, courseID, req.LectureNo).Scan(&lectureID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var completedAt any
	if req.Completed {
		completedAt = now
	}
	_, err = s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO lecture_progress(student_id, lecture_id, position_seconds, completed_at, last_studied_at)
		 VALUES($1,$2,$3,$4,$5)
		 ON CONFLICT(student_id, lecture_id) DO UPDATE SET
		   position_seconds = excluded.position_seconds,
		   last_studied_at = excluded.last_studied_at,
		   completed_at = COALESCE(excluded.completed_at, lecture_progress.completed_at)`,
		student.ID, lectureID, req.PositionSecond, completedAt, now)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "进度保存失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok", "completed": req.Completed}})
}
