package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func (s *Server) smsLogin(c *gin.Context) {
	var request struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Phone == "" || request.Code == "" {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil || s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
		return
	}
	student, err := s.identity.LoginBySMS(c.Request.Context(), request.Phone, request.Code, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "验证码错误或过期"})
		return
	}
	session, err := s.identity.CreateSessionWithDevice(c.Request.Context(), student.ID, c.Request.UserAgent(), c.ClientIP(), timeNowUTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "会话服务暂不可用"})
		return
	}
	c.SetCookie("cw_tc_sid", session.Token, int(timeUntil(session.ExpiresAt)), "/", "", true, true)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"student_no": student.StudentNo, "name": student.Name}})
}

func (s *Server) resetPassword(c *gin.Context) {
	var request struct {
		Phone       string `json:"phone"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Phone == "" || request.Code == "" || request.NewPassword == "" {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	if s.identity == nil || s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
		return
	}
	if err := s.identity.ResetPasswordBySMS(c.Request.Context(), request.Phone, request.Code, request.NewPassword, time.Now().UTC()); err != nil {
		if err == identity.ErrSMSInvalid {
			c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "验证码错误或过期"})
			return
		}
		c.JSON(http.StatusOK, envelope{Code: 10003, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
}

func (s *Server) rebindPhone(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var request struct {
		OldCode string `json:"old_code"`
		Phone   string `json:"phone"`
		NewCode string `json:"new_code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.OldCode == "" || request.Phone == "" || request.NewCode == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "请求参数非法"})
		return
	}
	if s.identity == nil || s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "短信服务暂不可用"})
		return
	}
	if err := s.identity.RebindPhone(c.Request.Context(), student.ID, request.OldCode, request.Phone, request.NewCode, time.Now().UTC()); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "ok"}})
}

func (s *Server) deleteAccount(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	if s.identity == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务暂不可用"})
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&input)
	if len([]rune(strings.TrimSpace(input.Reason))) > 500 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "注销原因过长"})
		return
	}
	// 注销需人工审核：先做进行中订单 / 退款前置校验。
	if reason, err := deletionPrecheck(c.Request.Context(), s.dbConn, student.ID); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销前置校验失败"})
		return
	} else if reason != "" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: reason})
		return
	}
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销申请提交失败"})
		return
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	var requestID int64
	if err := tx.QueryRowContext(c.Request.Context(),
		`INSERT INTO deletion_request(student_id, reason, status, created_at) VALUES($1,$2,'pending',$3) RETURNING id`,
		student.ID, strings.TrimSpace(input.Reason), now).Scan(&requestID); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "已提交过注销申请，请等待审核"})
		return
	}
	res, err := tx.ExecContext(c.Request.Context(),
		`UPDATE student SET status = 'deleting', updated_at = $1 WHERE id = $2 AND status = 'active'`, now, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销申请提交失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "当前账号状态不能申请注销"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销申请提交失败"})
		return
	}
	_ = s.identity.RevokeAllSessions(c.Request.Context(), student.ID)
	c.SetCookie("cw_tc_sid", "", -1, "/", "", true, true)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "student", ActorID: student.ID, Action: "account.deletion_request",
		TargetType: "student", TargetID: student.StudentNo, After: strings.TrimSpace(input.Reason), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "deleting", "request_id": requestID}})
}
