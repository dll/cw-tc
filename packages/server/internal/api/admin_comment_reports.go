package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// 评论下架原因枚举：**没有“差评”这一项**（PRD §9.10）。
var commentRemoveReasons = map[string]bool{
	"advertising": true,
	"abuse":       true,
	"privacy":     true,
	"illegal":     true,
	"irrelevant":  true,
	"other":       true,
}

func validCommentRemoveReason(value string) bool {
	return commentRemoveReasons[strings.TrimSpace(value)]
}

// adminHandleCommentReport POST /admin/comment-reports/:id/handle —— 处理举报并通知举报人。
func (s *Server) adminHandleCommentReport(c *gin.Context) {
	reportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || reportID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "举报编号非法"})
		return
	}
	var input struct {
		ResultNote    string `json:"result_note"`
		RemoveComment bool   `json:"remove_comment"`
		ReasonKind    string `json:"reason_kind"`
		Confirm       bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.ResultNote) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "处理举报必须填写结论并二次确认"})
		return
	}
	if input.RemoveComment && !validCommentRemoveReason(input.ReasonKind) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "下架评论必须选择违规原因（无差评选项）"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var commentID, reporterID int64
	var status string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT comment_id, reporter_id, status FROM comment_report WHERE id = $1`, reportID).
		Scan(&commentID, &reporterID, &status); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "举报不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "举报读取失败"})
		return
	}
	if status != "pending" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "举报已处理"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if input.RemoveComment {
		res, err := s.dbConn.ExecContext(c.Request.Context(),
			`UPDATE course_comment SET status='removed', remove_reason_kind=$1 WHERE id=$2 AND status='visible'`,
			input.ReasonKind, commentID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评论下架失败"})
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
				ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
				Action: "comment.remove", TargetType: "course_comment",
				TargetID: strconv.FormatInt(commentID, 10), After: input.ReasonKind, IP: c.ClientIP(),
			})
		}
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE comment_report SET status='handled', handled_by=$1, handled_at=$2, result_note=$3
		 WHERE id=$4 AND status='pending'`,
		admin.ID, now, strings.TrimSpace(input.ResultNote), reportID); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "举报处理失败"})
		return
	}
	s.pushNotice(c.Request.Context(), reporterID, "system", "举报已处理",
		strings.TrimSpace(input.ResultNote), "/courses")
	after, _ := json.Marshal(gin.H{"result_note": strings.TrimSpace(input.ResultNote), "remove_comment": input.RemoveComment})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "comment_report.handle", TargetType: "comment_report",
		TargetID: strconv.FormatInt(reportID, 10), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// adminRestoreComment POST /admin/comments/:id/restore —— 误判恢复（写审计）。
func (s *Server) adminRestoreComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "评论编号非法"})
		return
	}
	var input struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "恢复评论必须填写原因并二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE course_comment SET status='visible', remove_reason_kind=NULL WHERE id=$1 AND status='removed'`, id)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "评论恢复失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "评论不存在或无需恢复"})
		return
	}
	after, _ := json.Marshal(gin.H{"reason": strings.TrimSpace(input.Reason)})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "comment.restore", TargetType: "course_comment",
		TargetID: strconv.FormatInt(id, 10), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}
