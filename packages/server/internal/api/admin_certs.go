package api

import (
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

// adminRevokeCert POST /admin/certs/:certNo/revoke —— 撤销证书（须填原因 + 二次确认 + 写审计）。
//
// PRD §7.8 D-2：撤销要留痕；已撤销证书不可重复撤销，编号不可重用。
// 撤销只影响"对外有效性与下载"，不删除台账（编号仍可按验真页查询）。
func (s *Server) adminRevokeCert(c *gin.Context) {
	certNo := strings.TrimSpace(c.Param("certNo"))
	if certNo == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "证书编号非法"})
		return
	}
	var input struct {
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Reason) == "" || !input.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "撤销证书必须填写原因并二次确认"})
		return
	}
	if len([]rune(strings.TrimSpace(input.Reason))) > 500 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "原因过长（≤500 字）"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)

	var id, studentID, courseID int64
	var status string
	var revokedAt sql.NullString
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, student_id, course_id, status, revoked_at FROM certificate WHERE cert_no = $1`, certNo).
		Scan(&id, &studentID, &courseID, &status, &revokedAt); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "证书不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书读取失败"})
		return
	}
	if status == "revoked" || revokedAt.Valid {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "证书已撤销，不能重复撤销"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE certificate SET status = 'revoked', revoked_reason = 'admin', revoked_at = $1, revoked_by = $2
		 WHERE cert_no = $3 AND status = 'valid'`, now, admin.ID, certNo)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书撤销失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "证书已被撤销"})
		return
	}

	s.pushNotice(c.Request.Context(), studentID, "cert", "证书已撤销",
		"你的证书 "+certNo+" 已被撤销。原因："+strings.TrimSpace(input.Reason), "/certs")
	before, _ := json.Marshal(gin.H{"status": status})
	after, _ := json.Marshal(gin.H{
		"status": "revoked", "revoked_reason": "admin", "reason": strings.TrimSpace(input.Reason),
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "cert.revoke", TargetType: "certificate", TargetID: strconv.FormatInt(id, 10),
		Before: string(before), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"cert_no": certNo, "status": "revoked", "revoked_reason": "admin", "revoked_at": now,
		"student_id": studentID, "course_id": courseID,
	}})
}

// adminCertLedger GET /admin/certs —— 证书台账：状态 / 课程 / 编号前缀筛选，手机号脱敏。
func (s *Server) adminCertLedger(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	switch status {
	case "", "valid", "revoked":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 valid / revoked"})
		return
	}
	courseCode := strings.TrimSpace(c.Query("course_code"))
	qPattern := ""
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		qPattern = escapeLikePattern(q) + "%"
	}
	where := ` WHERE ($1 = '' OR c.status = $1)
	           AND ($2 = '' OR co.course_code = $2)
	           AND ($3 = '' OR c.cert_no LIKE $3 ESCAPE '\')`
	var total int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM certificate c JOIN course co ON co.id = c.course_id`+where,
		status, courseCode, qPattern).Scan(&total); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书台账读取失败"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.cert_no, s.phone, s.student_no, s.name, co.course_code, co.name,
		        c.level, c.score, c.band, c.status, c.issued_at, c.expires_at, c.revoked_reason, c.revoked_at
		 FROM certificate c
		 JOIN student s ON s.id = c.student_id
		 JOIN course co ON co.id = c.course_id`+where+
			` ORDER BY c.id DESC LIMIT $4 OFFSET $5`, status, courseCode, qPattern, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书台账读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var certNo, phone, studentNo, studentName, course, courseName, level, band, itemStatus, issuedAt, expiresAt string
		var score float64
		var revokedReason, revokedAt sql.NullString
		if err := rows.Scan(&certNo, &phone, &studentNo, &studentName, &course, &courseName,
			&level, &score, &band, &itemStatus, &issuedAt, &expiresAt, &revokedReason, &revokedAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书台账读取失败"})
			return
		}
		masked := ""
		if phone != "" {
			masked = redact.Phone(phone)
		}
		items = append(items, gin.H{
			"cert_no": certNo, "phone": masked, "student_no": studentNo, "student_name": studentName,
			"course": course, "course_name": courseName, "level": level, "score": score,
			"band": band, "status": itemStatus, "issued_at": issuedAt, "expires_at": expiresAt,
			"revoked_reason": nullableString(revokedReason), "revoked_at": nullableString(revokedAt),
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书台账读取失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "total": total}})
}
