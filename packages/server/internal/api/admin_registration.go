package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminPendingRegistrations 提供运营待办计数，直接查询学生事实表，避免通知丢失。
func (s *Server) adminPendingRegistrations(c *gin.Context) {
	var count int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM student WHERE status='active' AND registration_review_status='pending'`).Scan(&count); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "待办查询失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"count": count}})
}

// adminApproveRegistration POST /admin/users/:id/approve-registration。
// 手机号不走短信验证，注册后的可登录资格由运营管理员明确审核产生。
func (s *Server) adminApproveRegistration(c *gin.Context) {
	studentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || studentID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" || !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "审核事由与二次确认必填"})
		return
	}
	operator := c.MustGet(staffContextKey).(staff.Staff)
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "审核失败"})
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(c.Request.Context(), `UPDATE student SET phone_verified_at=$1, registration_review_status='approved', registration_reviewed_at=$1, registration_reviewed_by=$2, updated_at=$1 WHERE id=$3 AND status='active' AND registration_review_status='pending'`, now, operator.ID, studentID)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "审核失败"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "学员不存在或不可审核"})
		return
	}
	reasonJSON, _ := json.Marshal(map[string]string{"reason": strings.TrimSpace(req.Reason)})
	if err := audit.Record(c.Request.Context(), tx, audit.Input{ActorType: "staff", ActorID: operator.ID, ActorRole: string(operator.Role), Action: "student.registration_approve", TargetType: "student", TargetID: strconv.FormatInt(studentID, 10), After: string(reasonJSON), IP: c.ClientIP()}); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "审核失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: "审核失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "approved"}})
}
