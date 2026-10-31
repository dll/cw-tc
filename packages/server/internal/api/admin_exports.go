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
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

var exportScopes = map[string]bool{
	"account": true, "learning": true, "scores": true, "certs": true, "notes": true, "all": true,
}

// adminExportStudentData POST /admin/exports —— 代学员导出（仅 super · 事由必填 · 写审计与留痕）。
func (s *Server) adminExportStudentData(c *gin.Context) {
	var input struct {
		StudentID int64  `json:"student_id"`
		Scope     string `json:"scope"`
		Format    string `json:"format"`
		Reason    string `json:"reason"`
		Confirm   bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.StudentID <= 0 ||
		!exportScopes[input.Scope] || strings.TrimSpace(input.Reason) == "" || !input.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "代导出必须选择范围、填写事由并二次确认"})
		return
	}
	if input.Format == "" {
		input.Format = "json"
	}
	if input.Format != "json" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "当前仅支持 json 格式导出"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)

	var student identity.Student
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, student_no, name, phone FROM student WHERE id = $1`, input.StudentID).
		Scan(&student.ID, &student.StudentNo, &student.Name, &student.Phone); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "学员不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "学员读取失败"})
		return
	}
	data := filterExportScope(s.buildStudentExport(c.Request.Context(), student), input.Scope)
	var exportID int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO data_export(subject_type, subject_id, initiator, operator_id, reason, scope, format, created_at)
		 VALUES('student',$1,'admin',$2,$3,$4,$5,$6) RETURNING id`,
		student.ID, admin.ID, strings.TrimSpace(input.Reason), input.Scope, input.Format,
		time.Now().UTC().Format(time.RFC3339)).Scan(&exportID); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "导出留痕写入失败"})
		return
	}
	after, _ := json.Marshal(gin.H{"scope": input.Scope, "reason": strings.TrimSpace(input.Reason)})
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "student.export_admin", TargetType: "student", TargetID: strconv.FormatInt(student.ID, 10),
		After: string(after), IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"export_id": exportID, "scope": input.Scope, "format": input.Format, "data": data,
	}})
}

// adminListExports GET /admin/exports —— 导出留痕台账（仅 super）。
func (s *Server) adminListExports(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT d.id, d.subject_id, d.initiator, COALESCE(d.operator_id,0), COALESCE(d.reason,''),
		        d.scope, d.format, d.created_at, s.student_no, s.name
		 FROM data_export d LEFT JOIN student s ON s.id = d.subject_id
		 ORDER BY d.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "导出台账读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, subjectID, operatorID int64
		var initiator, reason, scope, format, createdAt string
		var studentNo, name sql.NullString
		if err := rows.Scan(&id, &subjectID, &initiator, &operatorID, &reason, &scope,
			&format, &createdAt, &studentNo, &name); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "导出台账读取失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "subject_id": subjectID, "student_no": nullableString(studentNo),
			"student_name": nullableString(name), "initiator": initiator, "operator_id": operatorID,
			"reason": reason, "scope": scope, "format": format, "created_at": createdAt,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
