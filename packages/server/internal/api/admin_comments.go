package api

import (
	"net/http"
	"strings"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func (s *Server) reportCourseComment(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		ReasonKind string `json:"reason_kind"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.ReasonKind) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "举报原因不能为空"})
		return
	}
	var commentID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT cc.id FROM course_comment cc WHERE cc.id=$1 AND cc.course_code=$2 AND cc.status='visible'`,
		c.Param("id"), c.Param("courseCode")).Scan(&commentID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "评价不存在"})
		return
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO comment_report(comment_id,reporter_id,reason_kind) VALUES($1,$2,$3) ON CONFLICT(comment_id,reporter_id) DO NOTHING`,
		commentID, student.ID, strings.TrimSpace(input.ReasonKind)); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "举报提交失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) adminListComments(c *gin.Context) {
	status := c.DefaultQuery("status", "visible")
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT cc.id, cc.course_code, co.name, cc.nickname_shown, cc.rating, COALESCE(cc.body,''), cc.status, cc.created_at,
		        (SELECT COUNT(*) FROM comment_report r WHERE r.comment_id=cc.id AND r.status='pending')
		 FROM course_comment cc JOIN course co ON co.course_code=cc.course_code
		 WHERE ($1='' OR cc.status=$1) ORDER BY cc.created_at DESC LIMIT 200`, status)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var code, name, nickname, statusValue, createdAt string
		var body string
		var rating, reports int
		if err := rows.Scan(&id, &code, &name, &nickname, &rating, &body, &statusValue, &createdAt, &reports); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价查询失败"})
			return
		}
		items = append(items, gin.H{"id": id, "course_code": code, "course_name": name,
			"nickname": nickname, "rating": rating, "body": body, "status": statusValue,
			"created_at": createdAt, "pending_reports": reports})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminRemoveComment(c *gin.Context) {
	var input struct {
		ReasonKind string `json:"reason_kind"`
		Confirm    bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.ReasonKind) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "下架评价需填写原因并二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course_comment SET status='removed', remove_reason_kind=$1 WHERE id=$2`,
		input.ReasonKind, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评价下架失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "评价不存在"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "comment.remove",
		TargetType: "course_comment", TargetID: c.Param("id"), After: input.ReasonKind, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) adminListCommentReports(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT r.id, r.comment_id, cc.course_code, cc.body, r.reason_kind, r.status, r.created_at
		 FROM comment_report r JOIN course_comment cc ON cc.id=r.comment_id
		 WHERE r.status='pending' ORDER BY r.created_at LIMIT 200`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "举报查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, commentID int64
		var code, body, reason, status, createdAt string
		if err := rows.Scan(&id, &commentID, &code, &body, &reason, &status, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "举报查询失败"})
			return
		}
		items = append(items, gin.H{"id": id, "comment_id": commentID, "course_code": code,
			"body": body, "reason_kind": reason, "status": status, "created_at": createdAt})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
