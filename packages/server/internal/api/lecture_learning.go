package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/redact"
	"github.com/gin-gonic/gin"
)

func (s *Server) learningLecture(c *gin.Context) (int64, int64, bool, bool) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "数据不可用"})
		return 0, 0, false, false
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil || no < 1 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "讲次编号无效"})
		return 0, 0, false, false
	}
	var courseID, lectureID int64
	var preview bool
	var previewCount int
	err = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT co.id,l.id,l.is_preview,co.free_preview_lectures FROM course co JOIN lecture l ON l.course_id=co.id WHERE co.course_code=$1 AND co.status='published' AND l.no=$2`, c.Param("courseCode"), no).Scan(&courseID, &lectureID, &preview, &previewCount)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讲次不存在"})
		return 0, 0, false, false
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "数据不可用"})
		return 0, 0, false, false
	}
	return courseID, lectureID, preview || no <= previewCount, true
}

func (s *Server) listLecturePractice(c *gin.Context) {
	courseID, lectureID, preview, ok := s.learningLecture(c)
	if !ok {
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if !s.checkResourceAccess(c, student.ID, courseID, preview) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT id,prompt,evidence_ref FROM lecture_question WHERE lecture_id=$1 ORDER BY CAST(source_key AS INTEGER),id`, lectureID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "练习不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var prompt, evidence string
		if err = rows.Scan(&id, &prompt, &evidence); err != nil {
			break
		}
		items = append(items, gin.H{"id": id, "prompt": prompt, "evidence_ref": evidence})
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "练习不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "grading": "self_review"}})
}

func (s *Server) submitLectureAnswer(c *gin.Context) {
	courseID, lectureID, preview, ok := s.learningLecture(c)
	if !ok {
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	if !s.checkResourceAccess(c, student.ID, courseID, preview) {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无学习权限"})
		return
	}
	questionID, err := strconv.ParseInt(c.Param("questionID"), 10, 64)
	if err != nil || questionID < 1 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "题目编号无效"})
		return
	}
	var req struct {
		Answer string `json:"answer"`
	}
	if c.ShouldBindJSON(&req) != nil || len(strings.TrimSpace(req.Answer)) == 0 || len([]rune(req.Answer)) > 5000 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "答案应为 1 至 5000 字"})
		return
	}
	var reference sql.NullString
	err = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT reference_answer FROM lecture_question WHERE id=$1 AND lecture_id=$2`, questionID, lectureID).Scan(&reference)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "题目不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "练习不可用"})
		return
	}
	_, err = s.dbConn.ExecContext(c.Request.Context(), `INSERT INTO lecture_answer(question_id,student_id,answer) VALUES($1,$2,$3)`, questionID, student.ID, strings.TrimSpace(req.Answer))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "保存失败"})
		return
	}
	score, graded := gradeLectureAnswer(req.Answer, reference.String)
	data := gin.H{"reference_answer": reference.String, "graded": graded}
	if graded {
		data["score"] = score
		data["grading"] = "auto"
		data["message"] = "已按参考答案给出参考相似度，仅供自查，不计入课程总测评或证书"
	} else {
		data["grading"] = "self_review"
		data["message"] = "本题暂无参考答案，请对照课程资料自行核对，本讲练习不计入总课程测评或证书"
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: data})
}

func discussionBodyOK(body string) bool {
	body = strings.TrimSpace(body)
	if len([]rune(body)) < 2 || len([]rune(body)) > 2000 || redact.ContainsIDCard(body) || redact.ContainsPhone(body) {
		return false
	}
	for _, word := range []string{"标准答案", "测评答案", "题库答案", "考试答案", "加微信", "加QQ"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(word)) {
			return false
		}
	}
	return true
}

// discussionExamLeak 在发布前比对总测评题干，避免讨论区泄露题目。
func (s *Server) discussionExamLeak(c *gin.Context, courseID int64, body string) bool {
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT stem FROM question WHERE course_id=$1`, courseID)
	if err != nil {
		return true
	}
	defer rows.Close()
	for rows.Next() {
		var stem string
		if rows.Scan(&stem) != nil {
			return true
		}
		stem = strings.TrimSpace(stem)
		if len([]rune(stem)) >= 12 && strings.Contains(body, stem) {
			return true
		}
	}
	return rows.Err() != nil
}

func publicDiscussionName(id int64) string { return "学员" + strconv.FormatInt(id%10000, 10) }

func (s *Server) listLectureTopics(c *gin.Context) {
	_, lectureID, _, ok := s.learningLecture(c)
	if !ok {
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT id,student_id,body,like_count,created_at FROM lecture_topic WHERE lecture_id=$1 ORDER BY like_count DESC,created_at DESC,id DESC LIMIT 100`, lectureID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, studentID int64
		var body, created string
		var likes int
		if err = rows.Scan(&id, &studentID, &body, &likes, &created); err != nil {
			break
		}
		items = append(items, gin.H{"id": id, "author": publicDiscussionName(studentID), "body": body, "likes": likes, "created_at": created})
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) createLectureTopic(c *gin.Context) {
	courseID, lectureID, _, ok := s.learningLecture(c)
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if c.ShouldBindJSON(&req) != nil || !discussionBodyOK(req.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "内容无效或含敏感信息"})
		return
	}
	if s.discussionExamLeak(c, courseID, req.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "内容疑似包含测评题目"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	_, err := s.dbConn.ExecContext(c.Request.Context(), `INSERT INTO lecture_topic(lecture_id,student_id,body) VALUES($1,$2,$3)`, lectureID, student.ID, strings.TrimSpace(req.Body))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "发布失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"published": true}})
}

func (s *Server) topicInLecture(c *gin.Context, lectureID int64) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("topicID"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "讨论编号无效"})
		return 0, false
	}
	var exists int
	err = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT 1 FROM lecture_topic WHERE id=$1 AND lecture_id=$2`, id, lectureID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "讨论不存在"})
		return 0, false
	}
	return id, true
}

func (s *Server) listLectureReplies(c *gin.Context) {
	_, lectureID, _, ok := s.learningLecture(c)
	if !ok {
		return
	}
	topicID, ok := s.topicInLecture(c, lectureID)
	if !ok {
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT id,student_id,body,created_at FROM lecture_reply WHERE topic_id=$1 ORDER BY created_at,id LIMIT 200`, topicID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评论不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, studentID int64
		var body, created string
		if err = rows.Scan(&id, &studentID, &body, &created); err != nil {
			break
		}
		items = append(items, gin.H{"id": id, "author": publicDiscussionName(studentID), "body": body, "created_at": created})
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评论不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) createLectureReply(c *gin.Context) {
	courseID, lectureID, _, ok := s.learningLecture(c)
	if !ok {
		return
	}
	topicID, ok := s.topicInLecture(c, lectureID)
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if c.ShouldBindJSON(&req) != nil || !discussionBodyOK(req.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "内容无效或含敏感信息"})
		return
	}
	if s.discussionExamLeak(c, courseID, req.Body) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "内容疑似包含测评题目"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	_, err := s.dbConn.ExecContext(c.Request.Context(), `INSERT INTO lecture_reply(topic_id,student_id,body) VALUES($1,$2,$3)`, topicID, student.ID, strings.TrimSpace(req.Body))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评论失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"published": true}})
}

func (s *Server) setLectureLike(c *gin.Context, liked bool) {
	_, lectureID, _, ok := s.learningLecture(c)
	if !ok {
		return
	}
	topicID, ok := s.topicInLecture(c, lectureID)
	if !ok {
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "点赞失败"})
		return
	}
	defer tx.Rollback()
	var result sql.Result
	if liked {
		result, err = tx.ExecContext(c.Request.Context(), `INSERT INTO lecture_topic_like(topic_id,student_id) VALUES($1,$2) ON CONFLICT(topic_id,student_id) DO NOTHING`, topicID, student.ID)
	} else {
		result, err = tx.ExecContext(c.Request.Context(), `DELETE FROM lecture_topic_like WHERE topic_id=$1 AND student_id=$2`, topicID, student.ID)
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "点赞失败"})
		return
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		delta := -1
		if liked {
			delta = 1
		}
		_, err = tx.ExecContext(c.Request.Context(), `UPDATE lecture_topic SET like_count=like_count+$1 WHERE id=$2`, delta, topicID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "点赞失败"})
			return
		}
	}
	if tx.Commit() != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "点赞失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"liked": liked}})
}

func (s *Server) likeLectureTopic(c *gin.Context)   { s.setLectureLike(c, true) }
func (s *Server) unlikeLectureTopic(c *gin.Context) { s.setLectureLike(c, false) }
