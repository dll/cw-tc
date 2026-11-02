package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// teacherDiscussions GET /teacher/discussions —— 本课讨论与问答工作台（只读自己的课）。
//
// 返回每帖回复数 / 最近活动 / 是否未回复，并附按课程的汇总（便于判断哪里需要答疑）。
func (s *Server) teacherDiscussions(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "limit 需在 1~200"})
		return
	}
	courseCode := strings.TrimSpace(c.Query("course_code"))
	onlyUnanswered := 0
	if c.Query("only_unanswered") == "true" {
		onlyUnanswered = 1
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT t.id, co.course_code, co.name, l.no, l.title, t.body,
		        st.student_no, st.name, t.like_count, t.created_at,
		        (SELECT COUNT(*) FROM lecture_reply r WHERE r.topic_id = t.id) +
		        (SELECT COUNT(*) FROM lecture_reply_staff rs WHERE rs.topic_id = t.id) AS reply_count,
		        CASE WHEN COALESCE((SELECT MAX(created_at) FROM lecture_reply r WHERE r.topic_id = t.id), '')
		                  >= COALESCE((SELECT MAX(created_at) FROM lecture_reply_staff rs WHERE rs.topic_id = t.id), '')
		             THEN COALESCE((SELECT MAX(created_at) FROM lecture_reply r WHERE r.topic_id = t.id), t.created_at)
		             ELSE COALESCE((SELECT MAX(created_at) FROM lecture_reply_staff rs WHERE rs.topic_id = t.id), t.created_at)
		        END AS last_activity
		 FROM lecture_topic t
		 JOIN lecture l ON l.id = t.lecture_id
		 JOIN course co ON co.id = l.course_id
		 LEFT JOIN student st ON st.id = t.student_id
		 WHERE co.teacher_id = $1 AND co.deleted_at IS NULL
		   AND ($2 = '' OR co.course_code = $2)
		   AND ($3 = 0 OR (
		        (SELECT COUNT(*) FROM lecture_reply r WHERE r.topic_id = t.id) +
		        (SELECT COUNT(*) FROM lecture_reply_staff rs WHERE rs.topic_id = t.id)) = 0)
		 ORDER BY last_activity DESC, t.id DESC LIMIT $4`,
		st.ID, courseCode, onlyUnanswered, limit)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var code, courseName, lectureTitle, body, createdAt, lastActivity string
		var lectureNo, likeCount, replyCount int
		var studentNo, studentName sql.NullString
		if err := rows.Scan(&id, &code, &courseName, &lectureNo, &lectureTitle, &body,
			&studentNo, &studentName, &likeCount, &createdAt, &replyCount, &lastActivity); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论读取失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "course_code": code, "course_name": courseName,
			"lecture_no": lectureNo, "lecture_title": lectureTitle,
			"body": body, "student_no": nullableString(studentNo), "student_name": nullableString(studentName),
			"likes": likeCount, "reply_count": replyCount, "unanswered": replyCount == 0,
			"created_at": createdAt, "last_activity": lastActivity,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论读取失败"})
		return
	}
	summaryRows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT co.course_code, co.name,
		        (SELECT COUNT(*) FROM lecture_topic t JOIN lecture l ON l.id = t.lecture_id WHERE l.course_id = co.id),
		        (SELECT COUNT(*) FROM lecture_topic t JOIN lecture l ON l.id = t.lecture_id
		          WHERE l.course_id = co.id AND (
		            (SELECT COUNT(*) FROM lecture_reply r WHERE r.topic_id = t.id) +
		            (SELECT COUNT(*) FROM lecture_reply_staff rs WHERE rs.topic_id = t.id)) = 0)
		 FROM course co
		 WHERE co.teacher_id = $1 AND co.deleted_at IS NULL AND ($2 = '' OR co.course_code = $2)
		 ORDER BY co.id`, st.ID, courseCode)
	summary := make([]gin.H, 0)
	if err == nil {
		defer summaryRows.Close()
		for summaryRows.Next() {
			var code, name string
			var total, unanswered int
			if summaryRows.Scan(&code, &name, &total, &unanswered) == nil {
				summary = append(summary, gin.H{
					"course_code": code, "course_name": name,
					"topics": total, "unanswered": unanswered,
				})
			}
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "summary": summary}})
}

// teacherReplyDiscussion POST /teacher/discussions/:id/replies —— 教师官方回复（不可匿名）。
func (s *Server) teacherReplyDiscussion(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	topicID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || topicID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "帖子编号非法"})
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !discussionBodyOK(input.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "回复内容不合法或包含敏感信息"})
		return
	}
	var courseID, studentID int64
	var courseCode string
	var lectureNo int
	var teacherID sql.NullInt64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT co.id, co.course_code, co.teacher_id, l.no, t.student_id
		 FROM lecture_topic t JOIN lecture l ON l.id = t.lecture_id JOIN course co ON co.id = l.course_id
		 WHERE t.id = $1 AND co.deleted_at IS NULL`, topicID).
		Scan(&courseID, &courseCode, &teacherID, &lectureNo, &studentID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "帖子不存在"})
		return
	}
	if !teacherID.Valid || teacherID.Int64 != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "只能回复本课讨论"})
		return
	}
	if s.discussionExamLeak(c, courseID, input.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "不得发布测评题目或答案"})
		return
	}
	var id int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO lecture_reply_staff(topic_id, staff_id, body, created_at) VALUES($1,$2,$3,$4) RETURNING id`,
		topicID, st.ID, strings.TrimSpace(input.Body), time.Now().UTC().Format(time.RFC3339)).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "回复失败"})
		return
	}
	if studentID > 0 {
		s.pushNotice(c.Request.Context(), studentID, "course", "教师回复了你的讨论",
			"《"+courseCode+"》第 "+strconv.Itoa(lectureNo)+" 讲的讨论有新回复", "/learn/"+courseCode+"/"+strconv.Itoa(lectureNo))
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: st.ID, ActorRole: string(st.Role),
		Action: "teacher.discussion_reply", TargetType: "lecture_topic", TargetID: strconv.FormatInt(topicID, 10),
		IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "official": true}})
}
