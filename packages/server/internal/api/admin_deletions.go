package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// deletionPrecheck 注销前置校验：不得有进行中订单 / 处理中退款。
func deletionPrecheck(ctx context.Context, db *sql.DB, studentID int64) (string, error) {
	var pendingOrders, processingRefunds int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders WHERE student_id = $1 AND status = 'pending'`, studentID).Scan(&pendingOrders); err != nil {
		return "", err
	}
	if pendingOrders > 0 {
		return "存在待支付订单，请先取消", nil
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM refund r JOIN orders o ON o.id = r.order_id
		 WHERE o.student_id = $1 AND r.status = 'processing'`, studentID).Scan(&processingRefunds); err != nil {
		return "", err
	}
	if processingRefunds > 0 {
		return "存在处理中退款，请等待完成", nil
	}
	return "", nil
}

// adminListDeletions GET /admin/deletions —— 注销待办（默认 pending）。
func (s *Server) adminListDeletions(c *gin.Context) {
	status := strings.TrimSpace(c.DefaultQuery("status", "pending"))
	switch status {
	case "", "pending", "approved", "rejected":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 pending / approved / rejected"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT d.id, d.student_id, d.reason, d.status, d.result_note, d.created_at, d.handled_at,
		        s.student_no, s.name, s.phone
		 FROM deletion_request d JOIN student s ON s.id = d.student_id
		 WHERE ($1 = '' OR d.status = $1)
		 ORDER BY CASE WHEN d.status = 'pending' THEN 0 ELSE 1 END, d.created_at DESC, d.id DESC
		 LIMIT 200`, status)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销待办读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, studentID int64
		var reason, itemStatus, createdAt, studentNo, name, phone string
		var resultNote, handledAt sql.NullString
		if err := rows.Scan(&id, &studentID, &reason, &itemStatus, &resultNote, &createdAt, &handledAt,
			&studentNo, &name, &phone); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销待办读取失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "student_id": studentID, "student_no": studentNo, "name": name,
			"phone_masked": redact.Phone(phone), "reason": reason, "status": itemStatus,
			"result_note": nullableString(resultNote), "created_at": createdAt,
			"handled_at": nullableString(handledAt),
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminHandleDeletion POST /admin/deletions/:id/approve —— 通过则匿名化，驳回则恢复账号。
func (s *Server) adminHandleDeletion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "注销申请编号非法"})
		return
	}
	var input struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.Note) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "处理注销必须填写结论并二次确认"})
		return
	}
	var studentID int64
	var status string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT student_id, status FROM deletion_request WHERE id = $1`, id).Scan(&studentID, &status); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "注销申请不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销申请读取失败"})
		return
	}
	if status != "pending" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "注销申请已处理"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	now := time.Now().UTC().Format(time.RFC3339)
	if input.Approve {
		if reason, err := deletionPrecheck(c.Request.Context(), s.dbConn, studentID); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销前置校验失败"})
			return
		} else if reason != "" {
			c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: reason})
			return
		}
		if s.identity == nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "账号服务不可用"})
			return
		}
		if err := s.identity.AnonymizeAccount(c.Request.Context(), studentID); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "匿名化执行失败"})
			return
		}
	} else {
		if _, err := s.dbConn.ExecContext(c.Request.Context(),
			`UPDATE student SET status = 'active', updated_at = $1 WHERE id = $2 AND status = 'deleting'`,
			now, studentID); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "恢复账号失败"})
			return
		}
		s.pushNotice(c.Request.Context(), studentID, "system", "注销申请未通过",
			strings.TrimSpace(input.Note), "/settings")
	}
	nextStatus := "rejected"
	if input.Approve {
		nextStatus = "approved"
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE deletion_request SET status = $1, handled_by = $2, handled_at = $3, result_note = $4
		 WHERE id = $5 AND status = 'pending'`,
		nextStatus, admin.ID, now, strings.TrimSpace(input.Note), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "注销申请更新失败"})
		return
	}
	after, _ := json.Marshal(gin.H{"approve": input.Approve, "note": strings.TrimSpace(input.Note)})
	action := "deletion.reject"
	if input.Approve {
		action = "deletion.approve"
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: action, TargetType: "deletion_request", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": nextStatus}})
}
