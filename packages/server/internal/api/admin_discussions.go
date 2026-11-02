package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func (s *Server) adminListDiscussions(c *gin.Context) {
	status := strings.TrimSpace(c.DefaultQuery("status", "visible"))
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT t.id, co.course_code, co.name, l.no, l.title, t.student_id, st.student_no, st.name,
		        t.body, t.status, COALESCE(t.remove_reason_kind,''), t.created_at,
		        (SELECT COUNT(*) FROM lecture_reply r WHERE r.topic_id=t.id AND r.status='visible') +
		        (SELECT COUNT(*) FROM lecture_reply_staff rs WHERE rs.topic_id=t.id AND rs.status='visible')
		 FROM lecture_topic t
		 JOIN lecture l ON l.id=t.lecture_id JOIN course co ON co.id=l.course_id JOIN student st ON st.id=t.student_id
		 WHERE ($1='' OR t.status=$1) ORDER BY t.created_at DESC, t.id DESC LIMIT 200`, status)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "讨论列表读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, studentID int64
		var courseCode, courseName, lectureTitle, studentNo, studentName, body, statusValue, reason, createdAt string
		var lectureNo, replies int
		if rows.Scan(&id, &courseCode, &courseName, &lectureNo, &lectureTitle, &studentID, &studentNo, &studentName,
			&body, &statusValue, &reason, &createdAt, &replies) == nil {
			items = append(items, gin.H{"id": id, "course_code": courseCode, "course_name": courseName,
				"lecture_no": lectureNo, "lecture_title": lectureTitle, "student_id": studentID,
				"student_no": studentNo, "student_name": studentName, "body": body, "status": statusValue,
				"remove_reason_kind": reason, "created_at": createdAt, "reply_count": replies})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminRemoveUGC POST /admin/ugc/:targetType/:id/remove —— 管理员下架讨论 / 回复（必填违规原因）。
func (s *Server) adminRemoveUGC(c *gin.Context) {
	var in struct {
		ReasonKind string `json:"reason_kind"`
		Confirm    bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !in.Confirm || !validCommentRemoveReason(in.ReasonKind) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "下架必须选择违规原因并二次确认（无差评选项）"})
		return
	}
	targetType := c.Param("targetType")
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "编号非法"})
		return
	}
	table, _, ok := ugcTable(targetType)
	if !ok {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "仅支持 topic / reply / teacher_reply"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var authorID int64
	var authorKind string
	switch targetType {
	case "topic":
		var studentID int64
		if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT student_id FROM lecture_topic WHERE id=$1`, id).Scan(&studentID); err != nil {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "帖子不存在"})
			return
		}
		authorID, authorKind = studentID, "student"
	case "reply":
		var studentID int64
		if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT student_id FROM lecture_reply WHERE id=$1`, id).Scan(&studentID); err != nil {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "回复不存在"})
			return
		}
		authorID, authorKind = studentID, "student"
	case "teacher_reply":
		var staffID int64
		if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT staff_id FROM lecture_reply_staff WHERE id=$1`, id).Scan(&staffID); err != nil {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "教师回复不存在"})
			return
		}
		authorID, authorKind = staffID, "staff"
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE `+table+` SET status='removed', remove_reason_kind=$1 WHERE id=$2 AND status='visible'`, in.ReasonKind, id)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "下架失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "内容不存在或已下架"})
		return
	}
	if authorKind == "student" && targetType != "teacher_reply" {
		s.pushNotice(c.Request.Context(), authorID, "system", "讨论内容已下架",
			"你的内容因违规已被下架："+in.ReasonKind, "/me/discussions")
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "ugc.remove",
		TargetType: targetType, TargetID: strconv.FormatInt(id, 10), After: in.ReasonKind, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) adminRestoreUGC(c *gin.Context) {
	targetType := c.Param("targetType")
	table, _, ok := ugcTable(targetType)
	if !ok {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "仅支持 topic / reply / teacher_reply"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in struct {
		Note    string `json:"note"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !in.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "恢复必须二次确认"})
		return
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE `+table+` SET status='visible', remove_reason_kind=NULL WHERE id=$1 AND status='removed'`, id)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "恢复失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "内容不存在或无需恢复"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "ugc.restore",
		TargetType: targetType, TargetID: strconv.FormatInt(id, 10), After: in.Note, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func ugcTable(targetType string) (string, string, bool) {
	switch targetType {
	case "topic":
		return "lecture_topic", "topic", true
	case "reply":
		return "lecture_reply", "reply", true
	case "teacher_reply":
		return "lecture_reply_staff", "teacher_reply", true
	default:
		return "", "", false
	}
}
