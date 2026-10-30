package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// myCourse 是「我的课程」列表项（PRD §7.13 L-1 / §7.4 B-5）。
type myCourse struct {
	Code            string `json:"course_code"`
	Name            string `json:"name"`
	MajorID         any    `json:"major_id"`
	MajorName       any    `json:"major_name"`
	Unlocked        bool   `json:"unlocked"`
	Status          string `json:"status"`
	LectureCount    int    `json:"lecture_count"`
	LearnedLectures int    `json:"learned_lectures"`
	ProgressPercent int    `json:"progress_percent"`
	LastLectureNo   any    `json:"last_lecture_no"`
	LastLearnedAt   any    `json:"last_learned_at"`
}

const myCourseProjection = `SELECT co.id, co.course_code, co.name, co.major_id, m.name, co.price_cents, co.status,
       COUNT(DISTINCT l.id),
       COUNT(DISTINCT CASE WHEN lp.completed_at IS NOT NULL THEN l.id END),
       (SELECT l2.no FROM lecture_progress p2 JOIN lecture l2 ON l2.id = p2.lecture_id
         WHERE p2.student_id = $1 AND l2.course_id = co.id
         ORDER BY p2.last_studied_at DESC LIMIT 1),
       MAX(lp.last_studied_at),
       EXISTS(SELECT 1 FROM entitlement e WHERE e.course_id = co.id AND e.student_id = $1 AND e.revoked_at IS NULL)
FROM course co
LEFT JOIN major m ON m.id = co.major_id
LEFT JOIN lecture l ON l.course_id = co.id
LEFT JOIN lecture_progress lp ON lp.lecture_id = l.id AND lp.student_id = $1`

// listMyCourses GET /me/courses —— 已发布课程 + 已解锁课程，含讲级进度。
func (s *Server) listMyCourses(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(), myCourseProjection+`
WHERE co.deleted_at IS NULL AND (co.status = 'published' OR EXISTS(
  SELECT 1 FROM entitlement e WHERE e.course_id = co.id AND e.student_id = $1 AND e.revoked_at IS NULL))
GROUP BY co.id, m.name
ORDER BY CASE WHEN MAX(lp.last_studied_at) IS NULL THEN 1 ELSE 0 END,
         MAX(lp.last_studied_at) DESC, co.name ASC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程查询失败"})
		return
	}
	defer rows.Close()
	items := make([]myCourse, 0)
	for rows.Next() {
		item, err := scanMyCourse(rows)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程查询失败"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程查询失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// scanMyCourse 读取一行投影并计算解锁状态 / 学习状态。
func scanMyCourse(scanner interface{ Scan(...any) error }) (myCourse, error) {
	var id, price int64
	var item myCourse
	var courseStatus string
	var majorID sql.NullInt64
	var majorName sql.NullString
	var lastNo sql.NullInt64
	var lastAt sql.NullString
	var entitled bool
	if err := scanner.Scan(&id, &item.Code, &item.Name, &majorID, &majorName, &price, &courseStatus,
		&item.LectureCount, &item.LearnedLectures, &lastNo, &lastAt, &entitled); err != nil {
		return myCourse{}, err
	}
	item.MajorID = nullableInt(majorID)
	item.MajorName = nullableString(majorName)
	item.Unlocked = entitled || (price == 0 && courseStatus == "published")
	item.Status = "locked"
	if item.Unlocked {
		item.Status = "learning"
		if item.LectureCount > 0 && item.LearnedLectures >= item.LectureCount {
			item.Status = "completed"
		}
	}
	item.ProgressPercent = progressPercent(item.LearnedLectures, item.LectureCount)
	if lastNo.Valid {
		item.LastLectureNo = lastNo.Int64
	}
	if lastAt.Valid {
		item.LastLearnedAt = lastAt.String
	}
	return item, nil
}

// reportCourseProgress PUT /me/courses/:code/progress —— 记录访问或显式标记本讲完成。
func (s *Server) reportCourseProgress(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	var req struct {
		LectureNo int   `json:"lecture_no"`
		Completed *bool `json:"completed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LectureNo < 1 || req.Completed == nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "进度参数无效"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	var courseID, lectureID, price int64
	var status string
	var preview bool
	var previewCount int
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT co.id, l.id, co.price_cents, co.status, l.is_preview, co.free_preview_lectures
		 FROM course co JOIN lecture l ON l.course_id = co.id
		 WHERE co.course_code = $1 AND co.deleted_at IS NULL AND l.no = $2`,
		c.Param("code"), req.LectureNo).
		Scan(&courseID, &lectureID, &price, &status, &preview, &previewCount)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程或讲次不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程服务不可用"})
		return
	}
	allowed := (status == "published" && (price == 0 || preview || req.LectureNo <= previewCount)) ||
		s.hasActiveEntitlement(c.Request.Context(), student.ID, courseID)
	if !allowed {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var completedAt any
	if *req.Completed {
		completedAt = now
	}
	_, err = s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO lecture_progress(student_id, lecture_id, completed_at, last_studied_at)
		 VALUES($1,$2,$3,$4)
		 ON CONFLICT(student_id, lecture_id) DO UPDATE SET
		   last_studied_at = excluded.last_studied_at,
		   completed_at = COALESCE(lecture_progress.completed_at, excluded.completed_at)`,
		student.ID, lectureID, completedAt, now)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "进度保存失败"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		myCourseProjection+` WHERE co.id = $2 GROUP BY co.id, m.name`, student.ID, courseID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "进度查询失败"})
		return
	}
	defer rows.Close()
	if !rows.Next() {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "进度查询失败"})
		return
	}
	item, err := scanMyCourse(rows)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "进度查询失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: item})
}

func progressPercent(learned, total int) int {
	if total <= 0 || learned <= 0 {
		return 0
	}
	if learned >= total {
		return 100
	}
	return learned * 100 / total
}
