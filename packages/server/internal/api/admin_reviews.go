package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const reviewSLAHours = 48

// adminReviewQueue GET /admin/reviews —— 审核待办队列（默认 submitted，含等待时长与 SLA 标记）。
func (s *Server) adminReviewQueue(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "limit 需在 1~100"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.course_code, c.name, c.source, c.status, c.content_version, c.submitted_at,
		        COALESCE(st.name,'') AS teacher_name, COALESCE(st.id,0) AS teacher_id
		 FROM course c LEFT JOIN staff st ON st.id = c.teacher_id
		 WHERE c.deleted_at IS NULL AND c.status = 'submitted'
		 ORDER BY c.submitted_at ASC, c.id ASC LIMIT $1`, limit)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "审核队列读取失败"})
		return
	}
	defer rows.Close()
	now := time.Now().UTC()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, source, status string
		var version, teacherID int64
		var submittedAt sql.NullString
		var teacherName string
		if err := rows.Scan(&code, &name, &source, &status, &version, &submittedAt, &teacherName, &teacherID); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "审核队列读取失败"})
			return
		}
		waitingHours := 0.0
		if submittedAt.Valid {
			if parsed, err := time.Parse(time.RFC3339, submittedAt.String); err == nil {
				waitingHours = now.Sub(parsed).Hours()
			}
		}
		items = append(items, gin.H{
			"course_code": code, "name": name, "source": source, "status": status,
			"content_version": version, "submitted_at": nullableString(submittedAt),
			"teacher_id": teacherID, "teacher_name": teacherName,
			"waiting_hours": waitingHours, "over_sla": waitingHours > reviewSLAHours,
			"sla_hours": reviewSLAHours,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items, "total": len(items)}})
}

// adminReviewDetail GET /admin/courses/:code/review —— 审核详情（内容快照 + 讲次资源 + 历史轮次）。
func (s *Server) adminReviewDetail(c *gin.Context) {
	code := c.Param("code")
	var id, teacherID, contentVersion int64
	var name, source, status string
	var submittedAt sql.NullString
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, name, source, status, content_version, submitted_at, COALESCE(teacher_id,0)
		 FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).
		Scan(&id, &name, &source, &status, &contentVersion, &submittedAt, &teacherID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "审核详情读取失败"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT l.no, l.title, l.is_preview,
		        (SELECT COUNT(*) FROM asset a WHERE a.lecture_id = l.id),
		        (SELECT COUNT(*) FROM lecture_question q WHERE q.lecture_id = l.id)
		 FROM lecture l WHERE l.course_id = $1 ORDER BY l.no`, id)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "审核详情读取失败"})
		return
	}
	lectures := make([]gin.H, 0)
	for rows.Next() {
		var no, assets, questions int
		var title string
		var preview bool
		if rows.Scan(&no, &title, &preview, &assets, &questions) == nil {
			lectures = append(lectures, gin.H{
				"no": no, "title": title, "is_preview": preview,
				"asset_count": assets, "question_count": questions,
			})
		}
	}
	rows.Close()

	questionRows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT level, COUNT(*) FROM question WHERE course_id = $1 GROUP BY level ORDER BY level`, id)
	questionsByLevel := make([]gin.H, 0)
	if err == nil {
		for questionRows.Next() {
			var level string
			var count int
			if questionRows.Scan(&level, &count) == nil {
				questionsByLevel = append(questionsByLevel, gin.H{"level": level, "count": count})
			}
		}
		questionRows.Close()
	}
	reviewRows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT round, content_version, reviewer_id, reviewer_role,
		        CAST(score_material AS REAL), CAST(score_objective AS REAL), CAST(score_case AS REAL),
		        CAST(score_lesson_plan AS REAL), CAST(score_slides AS REAL), CAST(score_lab AS REAL),
		        CAST(total_score AS REAL), verdict, COALESCE(CAST(issue_list AS TEXT),''), reviewed_at
		 FROM course_review WHERE course_code = $1 ORDER BY round DESC`, code)
	reviews := make([]gin.H, 0)
	if err == nil {
		for reviewRows.Next() {
			var round, version, reviewerID int64
			var role, verdict, issues, reviewedAt string
			var material, objective, caseScore, lesson, slides, lab, total float64
			if reviewRows.Scan(&round, &version, &reviewerID, &role, &material, &objective, &caseScore,
				&lesson, &slides, &lab, &total, &verdict, &issues, &reviewedAt) == nil {
				reviews = append(reviews, gin.H{
					"round": round, "content_version": version, "reviewer_id": reviewerID,
					"reviewer_role": role, "score_material": material, "score_objective": objective,
					"score_case": caseScore, "score_lesson_plan": lesson, "score_slides": slides,
					"score_lab": lab, "total_score": total, "verdict": verdict,
					"issue_list": issues, "reviewed_at": reviewedAt,
				})
			}
		}
		reviewRows.Close()
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"course_code": code, "name": name, "source": source, "status": status,
		"content_version": contentVersion, "submitted_at": nullableString(submittedAt),
		"teacher_id": teacherID, "lectures": lectures,
		"questions_by_level": questionsByLevel, "reviews": reviews,
		"checkup": gin.H{
			"lecture_count": len(lectures),
			"has_assets": func() bool {
				for _, item := range lectures {
					if item["asset_count"].(int) > 0 {
						return true
					}
				}
				return false
			}(),
			"question_levels": len(questionsByLevel),
		},
	}})
}
