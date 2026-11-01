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

// adminExamAnomalies GET /admin/exams/:id/anomalies —— 测评异常复核（只展示信号与人工结论）。
func (s *Server) adminExamAnomalies(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || sessionID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "测评会话编号非法"})
		return
	}
	var studentNo, studentName, courseCode, courseName, level, status string
	var score sql.NullFloat64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT st.student_no, st.name, co.course_code, co.name, es.level, es.status,
		        (SELECT CAST(er.total_score AS REAL) FROM exam_result er WHERE er.exam_session_id = es.id)
		 FROM exam_session es
		 JOIN student st ON st.id = es.student_id
		 JOIN course co ON co.id = es.course_id
		 WHERE es.id = $1`, sessionID).
		Scan(&studentNo, &studentName, &courseCode, &courseName, &level, &status, &score); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "测评会话不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评异常读取失败"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT a.id, a.kind, COALESCE(a.detail,''), a.at, a.verdict,
		        COALESCE(a.verdict_by,0), COALESCE(a.verdict_reason,''), COALESCE(a.verdict_at,''),
		        COALESCE(stf.name,'')
		 FROM exam_anomaly a LEFT JOIN staff stf ON stf.id = a.verdict_by
		 WHERE a.exam_id = $1 ORDER BY a.id`, sessionID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评异常读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, verdictBy int64
		var kind, detail, at, verdict, reason, verdictAt, reviewer string
		if err := rows.Scan(&id, &kind, &detail, &at, &verdict, &verdictBy, &reason, &verdictAt, &reviewer); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "测评异常读取失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "kind": kind, "detail": detail, "at": at, "verdict": verdict,
			"verdict_by": verdictBy, "verdict_reason": reason, "verdict_at": verdictAt,
			"reviewer_name": reviewer,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"exam_session_id": sessionID,
		"student_no":      studentNo, "student_name": studentName,
		"course_code": courseCode, "course_name": courseName,
		"level": level, "status": status, "total_score": nullableFloat(score),
		"items": items,
		"note":  "异常只是信号；判定必须由人完成并填写理由（AI 不参与判定，PRD §7.7 C-7）",
	}})
}

// adminExamVerdict POST /admin/exams/:id/verdict —— 人工复核结论（必填理由 + 二次确认 + 审计）。
func (s *Server) adminExamVerdict(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || sessionID <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "测评会话编号非法"})
		return
	}
	var input struct {
		Verdict   string `json:"verdict"`
		Reason    string `json:"reason"`
		AnomalyID int64  `json:"anomaly_id"`
		Confirm   bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !input.Confirm || strings.TrimSpace(input.Reason) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "复核结论必须填写理由并二次确认"})
		return
	}
	switch input.Verdict {
	case "kept", "invalid", "manual_check":
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "结论只接受 kept / invalid / manual_check"})
		return
	}
	var exists int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM exam_session WHERE id = $1`, sessionID).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "测评会话不存在"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE exam_anomaly SET verdict = $1, verdict_by = $2, verdict_at = $3, verdict_reason = $4
	          WHERE exam_id = $5 AND verdict = 'pending'`
	args := []any{input.Verdict, admin.ID, now, strings.TrimSpace(input.Reason), sessionID}
	if input.AnomalyID > 0 {
		query += ` AND id = $6`
		args = append(args, input.AnomalyID)
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "复核结论写入失败"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "没有待复核的异常记录"})
		return
	}
	after, _ := json.Marshal(gin.H{
		"verdict": input.Verdict, "reason": strings.TrimSpace(input.Reason),
		"anomaly_id": input.AnomalyID, "updated": affected,
	})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "exam_anomaly.verdict", TargetType: "exam_session",
		TargetID: strconv.FormatInt(sessionID, 10), After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"updated": affected}})
}

func nullableFloat(value sql.NullFloat64) any {
	if value.Valid {
		return value.Float64
	}
	return nil
}
