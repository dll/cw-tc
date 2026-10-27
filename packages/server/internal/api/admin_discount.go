package api

import (
	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

// adminBatchDiscount POST /admin/student-discounts —— 运营管理员批量设置指定课程的学员折扣。
func (s *Server) adminBatchDiscount(c *gin.Context) {
	var req struct {
		StudentIDs    []int64 `json:"student_ids"`
		CourseCode    string  `json:"course_code"`
		DiscountCents int64   `json:"discount_cents"`
		Reason        string  `json:"reason"`
		Confirm       bool    `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.StudentIDs) == 0 || req.CourseCode == "" || req.DiscountCents < 0 || req.Reason == "" || !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "学生、课程、折扣、事由及二次确认均必填"})
		return
	}
	var courseID int64
	var price int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT id,price_cents FROM course WHERE course_code=$1 AND status='published'`, req.CourseCode).Scan(&courseID, &price); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在或未上架"})
		return
	}
	if req.DiscountCents > price {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "折扣不能超过课程价格"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	count := 0
	for _, studentID := range req.StudentIDs {
		if studentID <= 0 {
			continue
		}
		if _, err = tx.ExecContext(c.Request.Context(), `UPDATE student_course_discount SET active=0 WHERE student_id=$1 AND course_id=$2 AND active=1`, studentID, courseID); err != nil {
			c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
			return
		}
		if _, err = tx.ExecContext(c.Request.Context(), `INSERT INTO student_course_discount(student_id,course_id,discount_cents,reason,granted_by,active,created_at) VALUES($1,$2,$3,$4,$5,1,$6)`, studentID, courseID, req.DiscountCents, req.Reason, admin.ID, now); err != nil {
			c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
			return
		}
		count++
	}
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "student.discount_batch", TargetType: "course", TargetID: strconv.FormatInt(courseID, 10), After: strconv.Itoa(count), IP: c.ClientIP()})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"updated": count}})
}

// adminBatchFree POST /admin/student-grants —— 批量授权免费学习，复用权益唯一约束与审计规则。
func (s *Server) adminBatchFree(c *gin.Context) {
	var req struct {
		StudentIDs []int64 `json:"student_ids"`
		CourseCode string  `json:"course_code"`
		Reason     string  `json:"reason"`
		Confirm    bool    `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.StudentIDs) == 0 || req.CourseCode == "" || req.Reason == "" || !req.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数及二次确认不完整"})
		return
	}
	var cid int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT id FROM course WHERE course_code=$1`, req.CourseCode).Scan(&cid); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	ok := 0
	for _, sid := range req.StudentIDs {
		if commerce.NewService(s.dbConn).GrantEntitlement(c.Request.Context(), sid, cid, admin.ID, req.Reason) == nil {
			ok++
		}
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "student.grant_batch", TargetType: "course", TargetID: strconv.FormatInt(cid, 10), After: strconv.Itoa(ok), IP: c.ClientIP()})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"granted": ok}})
}
