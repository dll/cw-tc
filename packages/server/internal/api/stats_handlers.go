package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/staff"
	"github.com/cw-tc/server/internal/stats"
	"github.com/cw-tc/server/internal/sysconfig"
	"github.com/gin-gonic/gin"
)

const statsRangeHelp = "range 仅支持 7d / 30d / 90d"

// publicCourseStats 把统计数字按展示阈值脱敏：低于阈值显示 null（前端渲染“少于 N 人”）。
func publicCourseStats(snapshot *stats.Snapshot, threshold int) gin.H {
	if threshold < sysconfig.MinDisplayThreshold {
		threshold = sysconfig.DefaultStatsDisplayThreshold
	}
	if snapshot == nil {
		return gin.H{
			"views_uv": nil, "favorites": nil, "learning": nil, "completed": nil,
			"examined": nil, "certified": nil,
			"data_as_of": nil, "display_threshold": threshold, "masked": true,
		}
	}
	mask := func(value int) any {
		if value < threshold {
			return nil
		}
		return value
	}
	payload := gin.H{
		"views_uv":          mask(snapshot.ViewsUV),
		"favorites":         mask(snapshot.Favorites),
		"learning":          mask(snapshot.Learning),
		"completed":         mask(snapshot.Completed),
		"examined":          mask(snapshot.Examined),
		"certified":         mask(snapshot.Certified),
		"data_as_of":        snapshot.StatDate,
		"display_threshold": threshold,
		"masked": snapshot.ViewsUV < threshold || snapshot.Favorites < threshold ||
			snapshot.Learning < threshold || snapshot.Completed < threshold ||
			snapshot.Examined < threshold || snapshot.Certified < threshold,
	}
	return payload
}

// staffCourseStats 是教师 / 管理员看到的精确统计（不脱敏，内部使用）。
func staffCourseStats(snapshot *stats.Snapshot, only string) gin.H {
	if snapshot == nil {
		return gin.H{
			"course_code": only, "data_as_of": nil,
			"views_uv": 0, "favorites": 0, "learning": 0, "completed": 0,
			"examined": 0, "certified": 0, "orders": 0, "paid": 0, "unlock": 0,
			"avg_rating": nil, "rating_count": 0,
		}
	}
	return gin.H{
		"course_code": snapshot.CourseCode, "data_as_of": snapshot.StatDate,
		"views_uv": snapshot.ViewsUV, "favorites": snapshot.Favorites,
		"learning": snapshot.Learning, "completed": snapshot.Completed,
		"examined": snapshot.Examined, "certified": snapshot.Certified,
		"orders": snapshot.Orders, "paid": snapshot.Paid, "unlock": snapshot.Unlock,
		"avg_rating": snapshot.AvgRating, "rating_count": snapshot.RatingCount,
		"computed_at": snapshot.ComputedAt,
	}
}

func funnelPayload(snapshot *stats.Snapshot) gin.H {
	if snapshot == nil {
		return gin.H{"items": []gin.H{}, "data_as_of": nil}
	}
	steps := []struct {
		key   string
		label string
		value int
	}{
		{"views_uv", "浏览", snapshot.ViewsUV},
		{"favorites", "收藏", snapshot.Favorites},
		{"orders", "下单", snapshot.Orders},
		{"paid", "支付解锁", snapshot.Paid},
		{"unlock", "开通权益", snapshot.Unlock},
		{"learning", "开始学习", snapshot.Learning},
		{"completed", "学完", snapshot.Completed},
		{"examined", "参加测评", snapshot.Examined},
		{"certified", "获证", snapshot.Certified},
	}
	items := make([]gin.H, 0, len(steps))
	previous := 0
	for i, step := range steps {
		var rate any
		if i > 0 && previous > 0 {
			rate = float64(step.value) / float64(previous)
		}
		items = append(items, gin.H{
			"key": step.key, "label": step.label, "value": step.value,
			"rate_from_previous": rate,
		})
		previous = step.value
	}
	return gin.H{"items": items, "data_as_of": snapshot.StatDate}
}

// recordCourseView 记录一次课程详情浏览；登录用户 24 小时内只算一次。
// 未登录无法识别“人”，按一次访问记一条（统计口径按天去重）。
func (s *Server) recordCourseView(c *gin.Context, code string) {
	if s.dbConn == nil {
		return
	}
	now := time.Now().UTC()
	var studentID int64
	if s.identity != nil {
		token := ""
		if cookie, err := c.Cookie("cw_tc_sid"); err == nil {
			token = cookie
		}
		if token == "" {
			if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
				token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			}
		}
		if token != "" {
			if student, err := s.identity.Authenticate(c.Request.Context(), token, now); err == nil {
				studentID = student.ID
			}
		}
	}
	ctx := c.Request.Context()
	if studentID > 0 {
		var seen int
		if err := s.dbConn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM course_view WHERE student_id = $1 AND course_code = $2 AND viewed_at >= $3`,
			studentID, code, now.Add(-24*time.Hour).Format(time.RFC3339)).Scan(&seen); err == nil && seen > 0 {
			return
		}
		_, _ = s.dbConn.ExecContext(ctx,
			`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES($1,$2,$3)`,
			code, studentID, now.Format(time.RFC3339))
		return
	}
	_, _ = s.dbConn.ExecContext(ctx,
		`INSERT INTO course_view(course_code, student_id, viewed_at) VALUES($1,NULL,$2)`,
		code, now.Format(time.RFC3339))
}

func (s *Server) statsThreshold(ctx context.Context) int {
	return sysconfig.New(s.dbConn).Int(ctx, sysconfig.KeyStatsDisplayThreshold, sysconfig.DefaultStatsDisplayThreshold)
}

func (s *Server) publicStatsFor(c *gin.Context, code string) gin.H {
	snapshot, err := stats.Latest(c.Request.Context(), s.dbConn, code)
	if err != nil {
		return publicCourseStats(nil, s.statsThreshold(c.Request.Context()))
	}
	return publicCourseStats(snapshot, s.statsThreshold(c.Request.Context()))
}

func (s *Server) teacherCourseStats(c *gin.Context) {
	st := c.MustGet(staffContextKey).(staff.Staff)
	code := c.Param("code")
	var teacherID sql.NullInt64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT teacher_id FROM course WHERE course_code = $1 AND deleted_at IS NULL`, code).
		Scan(&teacherID); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if !teacherID.Valid || teacherID.Int64 != st.ID {
		c.JSON(http.StatusForbidden, envelope{Code: 10703, Msg: "只能查看本课数据"})
		return
	}
	rangeKey := c.DefaultQuery("range", "7d")
	days, ok := rangeDays(rangeKey)
	if !ok {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: statsRangeHelp})
		return
	}
	snapshot, err := stats.Latest(c.Request.Context(), s.dbConn, code)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计查询失败"})
		return
	}
	payload := staffCourseStats(snapshot, code)
	payload["range"] = rangeKey
	payload["funnel"] = funnelPayload(snapshot)
	payload["series"] = s.statsSeries(c, code, days)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: payload})
}

func (s *Server) adminCourseStats(c *gin.Context) {
	code := c.Param("courseCode")
	snapshot, err := stats.Latest(c.Request.Context(), s.dbConn, code)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计查询失败"})
		return
	}
	payload := staffCourseStats(snapshot, code)
	payload["funnel"] = funnelPayload(snapshot)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: payload})
}

// adminInsights GET /admin/insights —— 全平台课程运营看板（逐课最新快照）。
func (s *Server) adminInsights(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计服务不可用"})
		return
	}
	query := `SELECT course_code, name, line, status, source FROM course WHERE deleted_at IS NULL`
	args := make([]any, 0, 2)
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		if source != "upstream" && source != "platform" {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "source 只接受 upstream / platform"})
			return
		}
		args = append(args, source)
		query += ` AND source = $1`
	}
	query += ` ORDER BY course_code`
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计服务不可用"})
		return
	}
	defer rows.Close()
	type courseRow struct{ code, name, line, status, source string }
	courses := make([]courseRow, 0)
	for rows.Next() {
		var row courseRow
		if err := rows.Scan(&row.code, &row.name, &row.line, &row.status, &row.source); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计服务不可用"})
			return
		}
		courses = append(courses, row)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计服务不可用"})
		return
	}
	rows.Close()

	items := make([]gin.H, 0, len(courses))
	for _, course := range courses {
		snapshot, err := stats.Latest(c.Request.Context(), s.dbConn, course.code)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "统计服务不可用"})
			return
		}
		item := staffCourseStats(snapshot, course.code)
		item["name"] = course.name
		item["line"] = course.line
		item["status"] = course.status
		item["source"] = course.source
		items = append(items, item)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) statsSeries(c *gin.Context, code string, days int) []stats.Snapshot {
	if days <= 0 {
		return []stats.Snapshot{}
	}
	to := stats.DateOf(time.Now().UTC())
	from := stats.DateOf(time.Now().UTC().Add(-time.Duration(days-1) * 24 * time.Hour))
	items, err := stats.Range(c.Request.Context(), s.dbConn, code, from, to)
	if err != nil || items == nil {
		return []stats.Snapshot{}
	}
	return items
}

func rangeDays(raw string) (int, bool) {
	switch raw {
	case "7d":
		return 7, true
	case "30d":
		return 30, true
	case "90d":
		return 90, true
	default:
		return 0, false
	}
}
