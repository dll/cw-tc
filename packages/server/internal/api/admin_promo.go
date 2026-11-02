package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func (s *Server) adminListPromo(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT p.code, p.kind, p.value, p.total_limit, p.per_student_limit, p.valid_from, p.valid_to, p.status,
		        (SELECT COUNT(*) FROM promo_redemption r WHERE r.promo_code=p.code),
		        (SELECT COALESCE(SUM(discount_cents),0) FROM promo_redemption r WHERE r.promo_code=p.code)
		 FROM promo_code p WHERE ($1='' OR p.status=$1) ORDER BY p.code`, strings.TrimSpace(c.Query("status")))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "优惠码读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, kind, from, to, status string
		var value int64
		var totalLimit, perStudent sql.NullInt64
		var used int
		var discounted int64
		if rows.Scan(&code, &kind, &value, &totalLimit, &perStudent, &from, &to, &status, &used, &discounted) == nil {
			var remaining any
			if totalLimit.Valid {
				remaining = totalLimit.Int64 - int64(used)
				if remaining.(int64) < 0 {
					remaining = int64(0)
				}
			}
			items = append(items, gin.H{"code": code, "kind": kind, "value": value,
				"total_limit": nullableInt64(totalLimit), "per_student_limit": nullableInt64(perStudent),
				"valid_from": from, "valid_to": to, "status": status, "used": used,
				"remaining": remaining, "discounted_cents": discounted})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminCreatePromo(c *gin.Context) {
	var in struct {
		Code            string `json:"code"`
		Kind            string `json:"kind"`
		Value           int64  `json:"value"`
		TotalLimit      *int64 `json:"total_limit"`
		PerStudentLimit *int64 `json:"per_student_limit"`
		ValidFrom       string `json:"valid_from"`
		ValidTo         string `json:"valid_to"`
		Confirm         bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !in.Confirm || strings.TrimSpace(in.Code) == "" ||
		(in.Kind != "amount_off" && in.Kind != "percent_off") || in.Value <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "优惠码参数不合法"})
		return
	}
	from, err1 := time.Parse(time.RFC3339, strings.TrimSpace(in.ValidFrom))
	to, err2 := time.Parse(time.RFC3339, strings.TrimSpace(in.ValidTo))
	if err1 != nil || err2 != nil || !from.Before(to) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "有效期格式非法或结束早于开始"})
		return
	}
	total := int64(-1)
	if in.TotalLimit != nil {
		if *in.TotalLimit < 0 {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "总量上限不能为负"})
			return
		}
		total = *in.TotalLimit
	}
	perStudent := int64(1)
	if in.PerStudentLimit != nil {
		if *in.PerStudentLimit <= 0 {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "单人限用必须大于 0"})
			return
		}
		perStudent = *in.PerStudentLimit
	}
	service := commerce.NewService(s.dbConn)
	if err := service.CreatePromo(c.Request.Context(), commerce.PromoInput{
		Code: strings.TrimSpace(in.Code), Kind: in.Kind, Value: in.Value, TotalLimit: total,
		PerStudentLimit: perStudent, ValidFrom: from, ValidTo: to,
	}); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "优惠码已存在或写入失败"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	_, _ = s.dbConn.ExecContext(c.Request.Context(), `UPDATE promo_code SET created_by=$1 WHERE code=$2`, admin.ID, strings.TrimSpace(in.Code))
	after, _ := json.Marshal(gin.H{"kind": in.Kind, "value": in.Value, "total_limit": total, "per_student_limit": perStudent})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "promo.create",
		TargetType: "promo_code", TargetID: strings.TrimSpace(in.Code), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"code": strings.TrimSpace(in.Code)}})
}

func (s *Server) adminUpdatePromo(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	var in struct {
		Status  string `json:"status"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !in.Confirm || (in.Status != "active" && in.Status != "disabled") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 active / disabled 且需二次确认"})
		return
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(), `UPDATE promo_code SET status=$1 WHERE code=$2`, in.Status, code)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "优惠码更新失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "优惠码不存在"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "promo.update",
		TargetType: "promo_code", TargetID: code, After: in.Status, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

// validatePromo POST /promo/validate —— 学员校验优惠码（不写库、不占额度）。
func (s *Server) validatePromo(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var in struct {
		Code       string `json:"code"`
		CourseCode string `json:"course_code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.CourseCode) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "优惠码与课程码必填"})
		return
	}
	var price int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT price_cents FROM course WHERE course_code=$1 AND status='published' AND deleted_at IS NULL`,
		in.CourseCode).Scan(&price); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在或未上架"})
		return
	}
	discount, err := commerce.NewService(s.dbConn).ValidatePromo(c.Request.Context(), in.Code, student.ID, price, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: err.Error()})
		return
	}
	if discount > price {
		discount = price
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"code": in.Code, "price_cents": price, "discount_cents": discount, "amount_cents": price - discount,
	}})
}

func nullableInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
