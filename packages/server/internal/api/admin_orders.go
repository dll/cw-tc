package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/commerce"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

type orderLedgerFilter struct {
	status     string
	courseCode string
	q          string
	from       string
	to         string
	limit      int
	offset     int
}

func parseOrderLedgerFilter(c *gin.Context) (orderLedgerFilter, bool) {
	var f orderLedgerFilter
	f.status = strings.TrimSpace(c.Query("status"))
	switch f.status {
	case "", "pending", "paid", "closed", "refunded":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 pending / paid / closed / refunded"})
		return f, false
	}
	f.courseCode = strings.TrimSpace(c.Query("course_code"))
	f.q = strings.TrimSpace(c.Query("q"))
	f.from = strings.TrimSpace(c.Query("from"))
	f.to = strings.TrimSpace(c.Query("to"))
	if f.from != "" {
		if _, err := time.Parse("2006-01-02", f.from); err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "from 格式应为 YYYY-MM-DD"})
			return f, false
		}
	}
	if f.to != "" {
		if _, err := time.Parse("2006-01-02", f.to); err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "to 格式应为 YYYY-MM-DD"})
			return f, false
		}
	}
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return f, false
	}
	f.limit, f.offset = limit, offset
	return f, true
}

func orderLedgerWhere(f orderLedgerFilter) (string, []any) {
	where := []string{"1=1"}
	args := make([]any, 0, 5)
	add := func(value any, clause string) {
		args = append(args, value)
		where = append(where, clause+"$"+strconv.Itoa(len(args)))
	}
	if f.status != "" {
		add(f.status, "o.status = ")
	}
	if f.courseCode != "" {
		add(f.courseCode, "c.course_code = ")
	}
	if f.q != "" {
		pattern := escapeLikePattern(f.q) + "%"
		args = append(args, pattern)
		idx := strconv.Itoa(len(args))
		where = append(where, "(o.order_no LIKE $"+idx+" ESCAPE '\\' OR s.phone LIKE $"+idx+" ESCAPE '\\' OR s.student_no LIKE $"+idx+" ESCAPE '\\')")
	}
	if f.from != "" {
		add(f.from, "o.created_at >= ")
	}
	if f.to != "" {
		end, _ := time.Parse("2006-01-02", f.to)
		add(end.Add(24*time.Hour).Format("2006-01-02"), "o.created_at < ")
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

// adminOrderLedger GET /admin/orders —— 订单查询（状态 / 课程 / 订单号或学员前缀 / 日期）。
func (s *Server) adminOrderLedger(c *gin.Context) {
	f, ok := parseOrderLedgerFilter(c)
	if !ok {
		return
	}
	where, args := orderLedgerWhere(f)
	var total int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM orders o JOIN student s ON s.id = o.student_id JOIN course c ON c.id = o.course_id`+where,
		args...).Scan(&total); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "订单查询失败"})
		return
	}
	listArgs := append(append([]any{}, args...), f.limit, f.offset)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT o.order_no, s.phone, s.student_no, c.course_code, o.course_name_snapshot,
		        o.amount_cents, o.status, COALESCE(o.channel,''), COALESCE(o.paid_at,''), o.created_at
		 FROM orders o JOIN student s ON s.id = o.student_id JOIN course c ON c.id = o.course_id`+where+
			fmt.Sprintf(` ORDER BY o.id DESC LIMIT $%d OFFSET $%d`, len(listArgs)-1, len(listArgs)), listArgs...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "订单查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		item, err := scanOrderLedger(rows)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "订单查询失败"})
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "total": total}})
}

// adminOrderExport GET /admin/orders/export —— 合规 CSV（手机号脱敏，不导出身份证）。
func (s *Server) adminOrderExport(c *gin.Context) {
	f, ok := parseOrderLedgerFilter(c)
	if !ok {
		return
	}
	f.limit, f.offset = 10000, 0
	where, args := orderLedgerWhere(f)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT o.order_no, s.phone, s.student_no, c.course_code, o.course_name_snapshot,
		        o.amount_cents, o.status, COALESCE(o.channel,''), COALESCE(o.paid_at,''), o.created_at
		 FROM orders o JOIN student s ON s.id = o.student_id JOIN course c ON c.id = o.course_id`+where+
			` ORDER BY o.id DESC LIMIT $`+strconv.Itoa(len(args)+1), append(args, 10000)...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "订单导出失败"})
		return
	}
	defer rows.Close()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="orders.csv"`)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"订单号", "手机号(脱敏)", "学号", "课程码", "课程", "金额(分)", "状态", "渠道", "支付时间", "下单时间"})
	for rows.Next() {
		item, err := scanOrderLedger(rows)
		if err != nil {
			break
		}
		_ = writer.Write([]string{
			fmt.Sprint(item["order_no"]), fmt.Sprint(item["phone"]), fmt.Sprint(item["student_no"]),
			fmt.Sprint(item["course_code"]), fmt.Sprint(item["course_name"]),
			fmt.Sprint(item["amount_cents"]), fmt.Sprint(item["status"]), fmt.Sprint(item["channel"]),
			fmt.Sprint(item["paid_at"]), fmt.Sprint(item["created_at"]),
		})
	}
	writer.Flush()
}

func scanOrderLedger(scanner interface{ Scan(...any) error }) (gin.H, error) {
	var orderNo, phone, studentNo, courseCode, courseName, status, channel, paidAt, createdAt string
	var amount int64
	if err := scanner.Scan(&orderNo, &phone, &studentNo, &courseCode, &courseName,
		&amount, &status, &channel, &paidAt, &createdAt); err != nil {
		return nil, err
	}
	return gin.H{
		"order_no": orderNo, "phone": redact.Phone(phone), "student_no": studentNo,
		"course_code": courseCode, "course_name": courseName, "amount_cents": amount,
		"status": status, "channel": channel, "paid_at": paidAt, "created_at": createdAt,
	}, nil
}

// adminPartialRefund POST /admin/orders/:orderNo/partial-refund —— 部分退款（= reward 打折，保留权益与证书）。
func (s *Server) adminPartialRefund(c *gin.Context) {
	var input struct {
		AmountCents int64  `json:"amount_cents"`
		Basis       string `json:"basis"`
		BasisRef    string `json:"basis_ref"`
		Reason      string `json:"reason"`
		Confirm     bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || input.AmountCents <= 0 ||
		strings.TrimSpace(input.Basis) == "" || strings.TrimSpace(input.BasisRef) == "" || strings.TrimSpace(input.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "部分退款必须填写金额、依据、事由并二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	refund, err := commerce.NewService(s.dbConn).Refund(c.Request.Context(), commerce.RefundInput{
		OrderNo: c.Param("orderNo"), Kind: "reward", Basis: strings.TrimSpace(input.Basis),
		BasisRef: strings.TrimSpace(input.BasisRef), AmountCents: input.AmountCents,
		Reason: strings.TrimSpace(input.Reason), Initiator: "admin", OperatorID: admin.ID,
	})
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	s.notifyOrderRefunded(c.Request.Context(), c.Param("orderNo"))
	after, _ := json.Marshal(gin.H{
		"kind": "reward", "amount_cents": input.AmountCents, "basis": input.Basis, "basis_ref": input.BasisRef,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "order.partial_refund", TargetType: "orders", TargetID: c.Param("orderNo"),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"refund_no": refund.RefundNo, "amount_cents": refund.AmountCents,
		"status": refund.Status, "kind": refund.Kind,
	}})
}
