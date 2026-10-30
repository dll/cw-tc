package api

import (
	"net/http"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

// exportMyData GET /me/export —— 学员自助导出（不含明文身份证号，写审计）。
func (s *Server) exportMyData(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	result := gin.H{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"account": gin.H{
			"student_no": student.StudentNo, "name": student.Name,
			"phone": maskPhone(student.Phone),
		},
		"notes":        []any{},
		"scores":       []any{},
		"certificates": []any{},
		"orders":       []any{},
		"suggestions":  []any{},
	}
	if rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT co.course_code, n.lecture_no, n.title, n.content_md, n.created_at FROM note n JOIN course co ON co.id=n.course_id WHERE n.student_id=$1 ORDER BY n.created_at`, student.ID); err == nil {
		items := make([]gin.H, 0)
		for rows.Next() {
			var code, title, body, createdAt string
			var lectureNo any
			if err := rows.Scan(&code, &lectureNo, &title, &body, &createdAt); err == nil {
				items = append(items, gin.H{"course_code": code, "lecture_no": lectureNo, "title": title, "body": body, "created_at": createdAt})
			}
		}
		rows.Close()
		result["notes"] = items
	}
	if rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT co.course_code, r.level, r.total_score, r.band, r.is_pass, r.graded_at
		 FROM exam_result r JOIN course co ON co.id=r.course_id WHERE r.student_id=$1 ORDER BY r.created_at`, student.ID); err == nil {
		items := make([]gin.H, 0)
		for rows.Next() {
			var code, level, band, gradedAt string
			var score float64
			var pass bool
			if err := rows.Scan(&code, &level, &score, &band, &pass, &gradedAt); err == nil {
				items = append(items, gin.H{"course_code": code, "level": level, "total_score": score, "band": band, "is_pass": pass, "graded_at": gradedAt})
			}
		}
		rows.Close()
		result["scores"] = items
	}
	if rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.cert_no, co.course_code, c.level, c.status, c.issued_at, c.expires_at
		 FROM certificate c JOIN course co ON co.id=c.course_id WHERE c.student_id=$1 ORDER BY c.issued_at`, student.ID); err == nil {
		items := make([]gin.H, 0)
		for rows.Next() {
			var certNo, code, level, status, issuedAt, expiresAt string
			if err := rows.Scan(&certNo, &code, &level, &status, &issuedAt, &expiresAt); err == nil {
				items = append(items, gin.H{"cert_no": certNo, "course_code": code, "level": level, "status": status, "issued_at": issuedAt, "expires_at": expiresAt})
			}
		}
		rows.Close()
		result["certificates"] = items
	}
	if rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT order_no, course_name_snapshot, amount_cents, status, created_at FROM orders WHERE student_id=$1 ORDER BY created_at`, student.ID); err == nil {
		items := make([]gin.H, 0)
		for rows.Next() {
			var orderNo, name, status, createdAt string
			var amount int64
			if err := rows.Scan(&orderNo, &name, &amount, &status, &createdAt); err == nil {
				items = append(items, gin.H{"order_no": orderNo, "course_name": name, "amount_cents": amount, "status": status, "created_at": createdAt})
			}
		}
		rows.Close()
		result["orders"] = items
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "student", ActorID: student.ID, Action: "student.export",
		TargetType: "student", TargetID: student.StudentNo, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: result})
}
