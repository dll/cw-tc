package api

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/sysconfig"
	"github.com/gin-gonic/gin"
)

// commentDisplayThreshold 是默认值；实际取值走 system_config（改后无需发版）。
const commentDisplayThreshold = sysconfig.DefaultCommentDisplayThreshold

var (
	commentContactRE = regexp.MustCompile(`(\b\d{17}[\dXx]\b)|(\b1[3-9]\d{9}\b)|(https?://)|(www\.)`)
	commentBanned    = []string{"包过", "保过", "代考", "给答案", "加微信", "加QQ"}
)

func validCommentBody(body string) bool {
	if body == "" {
		return true
	}
	if len([]rune(body)) < 20 || len([]rune(body)) > 500 {
		return false
	}
	if commentContactRE.MatchString(body) {
		return false
	}
	lower := strings.ToLower(body)
	for _, word := range commentBanned {
		if strings.Contains(lower, strings.ToLower(word)) {
			return false
		}
	}
	return true
}

// listCourseComments GET /courses/:courseCode/comments —— 公开评价（均分达阈值才展示）。
func (s *Server) listCourseComments(c *gin.Context) {
	code := c.Param("courseCode")
	var total int
	var avg sql.NullFloat64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*), AVG(CAST(rating AS REAL)) FROM course_comment WHERE course_code=$1 AND status='visible'`, code).
		Scan(&total, &avg); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价查询失败"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT nickname_shown, rating, rating_content, rating_level, rating_clarity, body,
		        completed_lectures, total_lectures, edited_at, created_at
		 FROM course_comment WHERE course_code=$1 AND status='visible'
		 ORDER BY created_at DESC LIMIT 50`, code)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var nickname, createdAt string
		var rating, completed, totalLectures int
		var body, editedAt sql.NullString
		var ratingContent, ratingLevel, ratingClarity sql.NullInt64
		if err := rows.Scan(&nickname, &rating, &ratingContent, &ratingLevel, &ratingClarity,
			&body, &completed, &totalLectures, &editedAt, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价查询失败"})
			return
		}
		items = append(items, gin.H{
			"nickname": nickname, "rating": rating, "body": nullableString(body),
			"rating_content": nullableInt(ratingContent), "rating_level": nullableInt(ratingLevel),
			"rating_clarity":     nullableInt(ratingClarity),
			"completed_lectures": completed, "total_lectures": totalLectures,
			"edited_at": nullableString(editedAt), "created_at": createdAt,
		})
	}
	threshold := sysconfig.New(s.dbConn).Int(c.Request.Context(),
		sysconfig.KeyCommentDisplayThreshold, sysconfig.DefaultCommentDisplayThreshold)
	data := gin.H{"items": items, "count": total, "display_threshold": threshold}
	if total >= threshold && avg.Valid {
		data["average"] = avg.Float64
	} else {
		data["average"] = nil
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: data})
}

// createCourseComment POST /courses/:courseCode/comments —— 已解锁学员一人一评。
func (s *Server) createCourseComment(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		Rating        int    `json:"rating"`
		RatingContent *int   `json:"rating_content"`
		RatingLevel   *int   `json:"rating_level"`
		RatingClarity *int   `json:"rating_clarity"`
		LectureNo     *int   `json:"lecture_no"`
		Body          string `json:"body"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Rating < 1 || input.Rating > 5 || !validCommentBody(input.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "评价内容不合规或参数错误"})
		return
	}
	var courseID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id FROM course WHERE course_code=$1 AND deleted_at IS NULL`, c.Param("courseCode")).Scan(&courseID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if !s.hasActiveEntitlement(c.Request.Context(), student.ID, courseID) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "购买 / 解锁后才能评价"})
		return
	}
	var completed, totalLectures int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM lecture WHERE course_id=$1`, courseID).Scan(&totalLectures)
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM lecture_progress p JOIN lecture l ON l.id=p.lecture_id
		 WHERE p.student_id=$1 AND l.course_id=$2 AND p.completed_at IS NOT NULL`, student.ID, courseID).Scan(&completed)
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO course_comment(course_code, student_id, rating, rating_content, rating_level, rating_clarity,
		 lecture_no, body, completed_lectures, total_lectures, nickname_shown, status)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'学员***','visible')`,
		c.Param("courseCode"), student.ID, input.Rating, input.RatingContent, input.RatingLevel, input.RatingClarity,
		input.LectureNo, nullableComment(input.Body), completed, totalLectures); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "每门课只能评价一次"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价保存失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func nullableComment(value string) any {
	if value == "" {
		return nil
	}
	return value
}
