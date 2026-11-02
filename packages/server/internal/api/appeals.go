package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

var appealKinds = map[string]bool{"ban": true, "identity": true, "score": true, "refund": true}

func validAppealContent(content string) bool {
	n := len([]rune(strings.TrimSpace(content)))
	return n >= 10 && n <= 2000
}

// createAnonymousAppeal POST /auth/appeal —— 登录页申诉（手机号 + 短信验证码）。
func (s *Server) createAnonymousAppeal(c *gin.Context) {
	var input struct {
		Phone       string `json:"phone"`
		Code        string `json:"code"`
		Kind        string `json:"kind"`
		ContentText string `json:"content_text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || identity.ValidatePhone(input.Phone) != nil ||
		input.Code == "" || !appealKinds[input.Kind] || !validAppealContent(input.ContentText) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "申诉参数不合法"})
		return
	}
	if s.sms == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉通道暂不可用"})
		return
	}
	now := time.Now().UTC()
	if err := s.sms.Consume(input.Phone, "appeal", input.Code, now); err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10101, Msg: "验证码错误或已过期"})
		return
	}
	// 限频：同一手机号 24h 内最多 5 次，防把申诉变成写库刷取口。
	var recent int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM account_appeal WHERE phone = $1 AND created_at >= $2`,
		input.Phone, now.Add(-24*time.Hour).Format(time.RFC3339)).Scan(&recent); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉提交失败"})
		return
	}
	if recent >= 5 {
		c.JSON(http.StatusTooManyRequests, envelope{Code: 10903, Msg: "申诉提交过于频繁，请稍后再试"})
		return
	}
	var studentID any
	var id int64
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id FROM student WHERE phone = $1`, input.Phone).Scan(&id)
	if id > 0 {
		studentID = id
	}
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO account_appeal(student_id, phone, kind, content_text, status, created_at)
		 VALUES($1,$2,$3,$4,'pending',$5) RETURNING id`,
		studentID, input.Phone, input.Kind, strings.TrimSpace(input.ContentText),
		now.Format(time.RFC3339)).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉提交失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": "pending"}})
}

// createMyAppeal POST /me/appeals —— 已登录学员提交申诉（改姓名 / 身份证、成绩、退款）。
func (s *Server) createMyAppeal(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var input struct {
		Kind        string `json:"kind"`
		ContentText string `json:"content_text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !appealKinds[input.Kind] || !validAppealContent(input.ContentText) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "申诉参数不合法"})
		return
	}
	var id int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO account_appeal(student_id, phone, kind, content_text, status, created_at)
		 VALUES($1,$2,$3,$4,'pending',$5) RETURNING id`,
		student.ID, student.Phone, input.Kind, strings.TrimSpace(input.ContentText),
		time.Now().UTC().Format(time.RFC3339)).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉提交失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": "pending"}})
}

// listMyAppeals GET /me/appeals —— 本人申诉与处理状态。
func (s *Server) listMyAppeals(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, kind, content_text, status, result_note, created_at, handled_at
		 FROM account_appeal WHERE student_id = $1 ORDER BY created_at DESC, id DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var kind, content, status, createdAt string
		var resultNote, handledAt sql.NullString
		if err := rows.Scan(&id, &kind, &content, &status, &resultNote, &createdAt, &handledAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉查询失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "kind": kind, "content_text": content, "status": status,
			"result_note": nullableString(resultNote), "created_at": createdAt,
			"handled_at": nullableString(handledAt),
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminListAppeals GET /admin/appeals —— 申诉待办（默认只看 pending）。
func (s *Server) adminListAppeals(c *gin.Context) {
	status := strings.TrimSpace(c.DefaultQuery("status", "pending"))
	switch status {
	case "", "pending", "accepted", "rejected":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 pending / accepted / rejected"})
		return
	}
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT a.id, a.phone, a.kind, a.content_text, a.status, a.result_note, a.created_at, a.handled_at,
		        s.student_no, s.name
		 FROM account_appeal a LEFT JOIN student s ON s.id = a.student_id
		 WHERE ($1 = '' OR a.status = $1)
		 ORDER BY CASE WHEN a.status = 'pending' THEN 0 ELSE 1 END, a.created_at DESC, a.id DESC
		 LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var phone, kind, content, itemStatus, createdAt string
		var resultNote, handledAt, studentNo, name sql.NullString
		if err := rows.Scan(&id, &phone, &kind, &content, &itemStatus, &resultNote,
			&createdAt, &handledAt, &studentNo, &name); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉查询失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "phone_masked": redact.Phone(phone), "kind": kind, "content_text": content,
			"status": itemStatus, "result_note": nullableString(resultNote),
			"student_no": nullableString(studentNo), "name": nullableString(name),
			"created_at": createdAt, "handled_at": nullableString(handledAt),
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminHandleAppeal POST /admin/appeals/:id/handle —— 处理申诉；封禁申诉通过时自动解封。
func (s *Server) adminHandleAppeal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "申诉编号非法"})
		return
	}
	var input struct {
		Status     string `json:"status"`
		ResultNote string `json:"result_note"`
		Confirm    bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm ||
		(input.Status != "accepted" && input.Status != "rejected") ||
		strings.TrimSpace(input.ResultNote) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "处理申诉必须填写结论并二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	var appealStatus, kind string
	var studentID sql.NullInt64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT status, kind, student_id FROM account_appeal WHERE id = $1`, id).
		Scan(&appealStatus, &kind, &studentID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "申诉不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉读取失败"})
		return
	}
	if appealStatus != "pending" {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "申诉已处理"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE account_appeal SET status = $1, result_note = $2, handled_by = $3, handled_at = $4
		 WHERE id = $5 AND status = 'pending'`,
		input.Status, strings.TrimSpace(input.ResultNote), admin.ID, now, id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "申诉处理失败"})
		return
	}
	// 封禁申诉通过：自动解封（不恢复已撤销证书 / 已退款订单）。
	if input.Status == "accepted" && kind == "ban" && studentID.Valid {
		if _, err := s.dbConn.ExecContext(c.Request.Context(),
			`UPDATE student SET status='active', status_reason_kind=NULL, status_note=NULL,
			        status_until=NULL, status_by=$1, updated_at=$2 WHERE id=$3`,
			admin.ID, now, studentID.Int64); err == nil {
			_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
				ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
				Action: "user.status_change", TargetType: "student", TargetID: strconv.FormatInt(studentID.Int64, 10),
				After: `{"status":"active","source":"appeal"}`, IP: c.ClientIP(),
			})
		}
		s.pushNotice(c.Request.Context(), studentID.Int64, "system", "申诉已受理",
			"你的账号申诉已通过："+strings.TrimSpace(input.ResultNote), "/me/appeals")
	} else if studentID.Valid {
		title := "申诉已驳回"
		if input.Status == "accepted" {
			title = "申诉已受理"
		}
		s.pushNotice(c.Request.Context(), studentID.Int64, "system", title,
			strings.TrimSpace(input.ResultNote), "/me/appeals")
	}
	after, _ := json.Marshal(gin.H{"status": input.Status, "result_note": strings.TrimSpace(input.ResultNote)})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "account_appeal.handle", TargetType: "account_appeal", TargetID: strconv.FormatInt(id, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": input.Status}})
}
