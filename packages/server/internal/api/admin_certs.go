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
