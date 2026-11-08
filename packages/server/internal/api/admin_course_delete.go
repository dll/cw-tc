package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminDeleteCourse DELETE /admin/courses/:code —— 软删除平台课程（高危，二次确认 + 事由 + 审计）。
//
// 只允许删除 platform 课程，且必须没有订单 / 权益 / 测评 / 证书等业务数据；
// 已上架课程必须先下架。不物理删除（沿用 course_delete_log 留痕）。
func (s *Server) adminDeleteCourse(c *gin.Context) {
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var input struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "删除课程必须填写事由并二次确认"})
		return
	}
	code := strings.TrimSpace(c.Param("code"))
	var courseID int64
	var name, source, status string
	err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, name, source, status FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).
		Scan(&courseID, &name, &source, &status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在或已删除"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程读取失败"})
		return
	}
	if source != "platform" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "上游课程由同步维护，不能在后台删除"})
		return
	}
	if status == "published" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "已上架课程请先下架，再删除"})
		return
	}
	for _, check := range []struct {
		table string
		label string
	}{
		{"orders", "订单"}, {"entitlement", "学习权益"}, {"certificate", "证书"},
		{"exam_session", "测评记录"}, {"exam_result", "测评成绩"},
	} {
		var count int
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT COUNT(*) FROM `+check.table+` WHERE course_id = $1`, courseID).Scan(&count); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程关联数据检查失败"})
			return
		}
		if count > 0 {
			c.JSON(http.StatusConflict, envelope{Code: 10409,
				Msg: "课程已有" + check.label + "，不能删除；请下架保留"})
			return
		}
	}

	now := time.Now().UTC()
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程删除失败"})
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(c.Request.Context(),
		`UPDATE course SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL`,
		now, courseID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程删除失败"})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "课程已被删除"})
		return
	}
	if _, err := tx.ExecContext(c.Request.Context(),
		`INSERT INTO course_delete_log(course_code, course_name, deleted_by, deleted_at) VALUES($1,$2,$3,$4)`,
		code, name, admin.ID, now); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程删除失败"})
		return
	}
	// 相关运营位同步置为 removed，避免已删除课程继续出现在首页 / 广场。
	if _, err := tx.ExecContext(c.Request.Context(),
		`UPDATE operation_slot SET status = 'removed' WHERE course_code = $1 AND status <> 'removed'`, code); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程删除失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程删除失败"})
		return
	}

	before, _ := json.Marshal(gin.H{"course_code": code, "name": name, "status": status})
	after, _ := json.Marshal(gin.H{"deleted_at": now.Format(time.RFC3339), "reason": strings.TrimSpace(input.Reason)})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "course.delete", TargetType: "course", TargetID: code,
		Before: string(before), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"course_code": code}})
}
