package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/redact"
	"github.com/gin-gonic/gin"
)

// adminUserDetail GET /admin/users/:id —— 用户详情（默认脱敏，PRD §7.10.10（二））。
//
// 学员详情包含学习进度、测评、证书、订单与 AI 用量；**笔记永不返回**。
// 学员与内部人员的 id 各自独立，调用方用 `type` 指定角色（默认 student）。
func (s *Server) adminUserDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "用户编号非法"})
		return
	}
	userType := strings.TrimSpace(c.DefaultQuery("type", "student"))
	switch userType {
	case "student":
		s.adminStudentDetail(c, id)
	case "teacher", "admin_super", "admin_ops":
		s.adminStaffDetail(c, id, userType)
	default:
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "用户类型非法"})
	}
}

func (s *Server) adminStudentDetail(c *gin.Context, id int64) {
	ctx := c.Request.Context()
	var studentNo, name, status, createdAt string
	var phone sql.NullString
	var reviewStatus, phoneVerified, lastLogin sql.NullString
	if err := s.dbConn.QueryRowContext(ctx,
		`SELECT student_no, name, phone, status, created_at, registration_review_status, phone_verified_at, last_login_at
		 FROM student WHERE id = $1`, id).
		Scan(&studentNo, &name, &phone, &status, &createdAt, &reviewStatus, &phoneVerified, &lastLogin); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "用户不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "用户详情读取失败"})
		return
	}
	maskedPhone := ""
	if phone.Valid && phone.String != "" {
		maskedPhone = redact.Phone(phone.String)
	}
	maskedIDCard := ""
	if s.identity != nil {
		if masked, err := s.identity.MaskedIDCard(ctx, id); err == nil {
			maskedIDCard = masked
		}
	}

	learning := s.studentLearningCourses(c, id)
	completedCourses := 0
	for _, item := range learning {
		if item["progress_percent"] == 100 {
			completedCourses++
		}
	}

	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"user_type":           "student",
		"id":                  id,
		"student_no":          studentNo,
		"name":                name,
		"phone_masked":        maskedPhone,
		"id_card_masked":      maskedIDCard,
		"status":              status,
		"created_at":          createdAt,
		"registration_status": nullableString(reviewStatus),
		"phone_verified_at":   nullableString(phoneVerified),
		"last_login_at":       nullableString(lastLogin),
		"learning":            learning,
		"learning_summary": gin.H{
			"courses":           len(learning),
			"completed_courses": completedCourses,
		},
		"exams":        s.studentExamRecords(c, id),
		"certificates": s.studentCertificates(c, id),
		"orders":       s.studentOrderRecords(c, id),
		"ai_usage":     s.studentAIUsage(c, id),
		"note":         "学习笔记仅本人可见，管理员不读取（PRD §7.6）",
	}})
}

func (s *Server) studentLearningCourses(c *gin.Context, studentID int64) []gin.H {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.course_code, c.name, c.status,
		        (SELECT COUNT(*) FROM lecture l WHERE l.course_id = c.id) AS lecture_count,
		        (SELECT COUNT(*) FROM lecture_progress p JOIN lecture l ON l.id = p.lecture_id
		          WHERE p.student_id = $1 AND l.course_id = c.id AND p.completed_at IS NOT NULL) AS learned_lectures,
		        (SELECT MAX(p.last_studied_at) FROM lecture_progress p JOIN lecture l ON l.id = p.lecture_id
		          WHERE p.student_id = $1 AND l.course_id = c.id) AS last_learned_at,
		        (SELECT l2.no FROM lecture_progress p2 JOIN lecture l2 ON l2.id = p2.lecture_id
		          WHERE p2.student_id = $1 AND l2.course_id = c.id ORDER BY p2.last_studied_at DESC LIMIT 1) AS last_lecture_no,
		        EXISTS(SELECT 1 FROM entitlement e WHERE e.student_id = $1 AND e.course_id = c.id AND e.revoked_at IS NULL) AS unlocked
		 FROM course c
		 WHERE c.deleted_at IS NULL
		   AND (EXISTS(SELECT 1 FROM lecture_progress p3 JOIN lecture l3 ON l3.id = p3.lecture_id
		                WHERE p3.student_id = $1 AND l3.course_id = c.id)
		        OR EXISTS(SELECT 1 FROM entitlement e2 WHERE e2.student_id = $1 AND e2.course_id = c.id AND e2.revoked_at IS NULL))
		 ORDER BY last_learned_at DESC, c.id DESC`, studentID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, status string
		var lectureCount, learned int
		var lastAt sql.NullString
		var lastNo sql.NullInt64
		var unlocked bool
		if err := rows.Scan(&code, &name, &status, &lectureCount, &learned, &lastAt, &lastNo, &unlocked); err != nil {
			return []gin.H{}
		}
		items = append(items, gin.H{
			"course_code":      code,
			"name":             name,
			"course_status":    status,
			"lecture_count":    lectureCount,
			"learned_lectures": learned,
			"progress_percent": progressPercent(learned, lectureCount),
			"last_lecture_no":  nullableInt(lastNo),
			"last_learned_at":  nullableString(lastAt),
			"unlocked":         unlocked,
		})
	}
	return items
}

func (s *Server) studentExamRecords(c *gin.Context, studentID int64) []gin.H {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.course_code, c.name, er.level, er.total_score, er.band, er.is_pass, er.created_at,
		        (SELECT ce.cert_no FROM certificate ce
		          WHERE ce.student_id = er.student_id AND ce.course_id = er.course_id
		            AND ce.level = er.level AND ce.revoked_at IS NULL
		          ORDER BY ce.issued_at DESC LIMIT 1) AS cert_no
		 FROM exam_result er JOIN course c ON c.id = er.course_id
		 WHERE er.student_id = $1
		 ORDER BY er.created_at DESC, er.id DESC LIMIT 50`, studentID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, level, createdAt string
		var score float64
		var band, certNo sql.NullString
		var passed bool
		if err := rows.Scan(&code, &name, &level, &score, &band, &passed, &createdAt, &certNo); err != nil {
			return []gin.H{}
		}
		items = append(items, gin.H{
			"course_code": code, "course_name": name, "level": level,
			"total_score": score, "band": nullableString(band), "is_pass": passed,
			"exam_at": createdAt, "cert_no": nullableString(certNo),
		})
	}
	return items
}

func (s *Server) studentCertificates(c *gin.Context, studentID int64) []gin.H {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT ce.cert_no, c.course_code, c.name, ce.level, ce.score, ce.band,
		        ce.issued_at, ce.expires_at, ce.status, ce.revoked_reason
		 FROM certificate ce JOIN course c ON c.id = ce.course_id
		 WHERE ce.student_id = $1
		 ORDER BY ce.issued_at DESC, ce.id DESC LIMIT 50`, studentID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var certNo, code, name, level, band, issuedAt, expiresAt, status string
		var score float64
		var revokedReason sql.NullString
		if err := rows.Scan(&certNo, &code, &name, &level, &score, &band,
			&issuedAt, &expiresAt, &status, &revokedReason); err != nil {
			return []gin.H{}
		}
		items = append(items, gin.H{
			"cert_no": certNo, "course_code": code, "course_name": name,
			"level": level, "score": score, "band": band,
			"issued_at": issuedAt, "expires_at": expiresAt,
			"status": status, "revoked_reason": nullableString(revokedReason),
		})
	}
	return items
}

func (s *Server) studentOrderRecords(c *gin.Context, studentID int64) []gin.H {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT order_no, course_name_snapshot, amount_cents, status, COALESCE(channel,''),
		        created_at, paid_at, refunded_at
		 FROM orders WHERE student_id = $1
		 ORDER BY created_at DESC, id DESC LIMIT 50`, studentID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var orderNo, courseName, status, channel, createdAt string
		var amount int64
		var paidAt, refundedAt sql.NullString
		if err := rows.Scan(&orderNo, &courseName, &amount, &status, &channel,
			&createdAt, &paidAt, &refundedAt); err != nil {
			return []gin.H{}
		}
		items = append(items, gin.H{
			"order_no": orderNo, "course_name": courseName, "amount_cents": amount,
			"status": status, "channel": channel, "created_at": createdAt,
			"paid_at": nullableString(paidAt), "refunded_at": nullableString(refundedAt),
		})
	}
	return items
}

func (s *Server) studentAIUsage(c *gin.Context, studentID int64) gin.H {
	var used int
	var courseCount int
	var lastDate sql.NullString
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COALESCE(SUM(used),0), COUNT(DISTINCT course_id), MAX(stat_date)
		 FROM ai_daily_quota WHERE student_id = $1`, studentID).
		Scan(&used, &courseCount, &lastDate); err != nil {
		return gin.H{"used_total": 0, "courses": 0, "last_date": nil}
	}
	return gin.H{"used_total": used, "courses": courseCount, "last_date": nullableString(lastDate)}
}

func (s *Server) adminStaffDetail(c *gin.Context, id int64, expectedType string) {
	var account, name, role, status, createdAt string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT account, name, role, status, created_at FROM staff WHERE id = $1 AND role = $2`, id, expectedType).
		Scan(&account, &name, &role, &status, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "用户不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "用户详情读取失败"})
		return
	}
	data := gin.H{
		"user_type": role, "id": id, "account": account, "name": name,
		"status": status, "created_at": createdAt,
	}
	if role == "teacher" {
		rows, err := s.dbConn.QueryContext(c.Request.Context(),
			`SELECT course_code, name, status FROM course
			 WHERE teacher_id = $1 AND deleted_at IS NULL ORDER BY id DESC`, id)
		if err == nil {
			defer rows.Close()
			courses := make([]gin.H, 0)
			for rows.Next() {
				var code, courseName, courseStatus string
				if rows.Scan(&code, &courseName, &courseStatus) == nil {
					courses = append(courses, gin.H{"course_code": code, "name": courseName, "status": courseStatus})
				}
			}
			data["courses"] = courses
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: data})
}
