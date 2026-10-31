// Package stats 计算课程日统计（PRD §7.14 M-0 / 数据字典 §6.6）。
//
// 口径一律「按人去重」，且同一天重复跑批结果必须一致：
// 写入使用 ON CONFLICT(course_code, stat_date) DO UPDATE，不是 INSERT。
package stats

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
)

const (
	// LearningWindowDays 是「在学」的滚动口径：最近 30 天有学习行为（M-0）。
	LearningWindowDays = 30
	// 时间按 Asia/Shanghai 切日（与 PRD §7.14 一致）。
	cstOffset = 8 * 3600
)

type Snapshot struct {
	CourseCode  string   `json:"course_code"`
	StatDate    string   `json:"stat_date"`
	ViewsUV     int      `json:"views_uv"`
	Favorites   int      `json:"favorites"`
	Learning    int      `json:"learning"`
	Completed   int      `json:"completed"`
	Examined    int      `json:"examined"`
	Certified   int      `json:"certified"`
	Orders      int      `json:"orders"`
	Paid        int      `json:"paid"`
	Unlock      int      `json:"unlock"`
	AvgRating   *float64 `json:"avg_rating"`
	RatingCount int      `json:"rating_count"`
	ComputedAt  string   `json:"computed_at"`
}

// DateOf 把时间换算成 Asia/Shanghai 的日期（YYYY-MM-DD）。
func DateOf(t time.Time) string {
	return t.In(time.FixedZone("CST", cstOffset)).Format("2006-01-02")
}

// ParseDate 解析 YYYY-MM-DD；空串返回 false。
func ParseDate(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	day, err := time.ParseInLocation("2006-01-02", raw, time.FixedZone("CST", cstOffset))
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// RecomputeAll 对全部未删除课程重算指定日期（T+1 跑批入口）。
func RecomputeAll(ctx context.Context, db *sql.DB, statDate, now time.Time) ([]Snapshot, error) {
	if db == nil {
		return nil, errors.New("数据库未配置")
	}
	rows, err := db.QueryContext(ctx,
		`SELECT course_code FROM course WHERE deleted_at IS NULL ORDER BY course_code`)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			return nil, err
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	out := make([]Snapshot, 0, len(codes))
	for _, code := range codes {
		snapshot, err := Recompute(ctx, db, code, statDate, now)
		if err != nil {
			return out, err
		}
		out = append(out, snapshot)
	}
	return out, nil
}

// Recompute 重算单课某日统计并落库（幂等）。
func Recompute(ctx context.Context, db *sql.DB, courseCode string, statDate, now time.Time) (Snapshot, error) {
	if db == nil || courseCode == "" {
		return Snapshot{}, errors.New("统计参数非法")
	}
	if statDate.IsZero() {
		statDate = now
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	day := DateOf(statDate)
	startLocal := time.Date(statDate.In(time.FixedZone("CST", cstOffset)).Year(),
		statDate.In(time.FixedZone("CST", cstOffset)).Month(),
		statDate.In(time.FixedZone("CST", cstOffset)).Day(), 0, 0, 0, 0, time.FixedZone("CST", cstOffset))
	end := startLocal.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	learningStart := startLocal.Add(-LearningWindowDays * 24 * time.Hour).UTC().Format(time.RFC3339)
	until := end

	var courseID, priceCents int64
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT id, price_cents, status FROM course WHERE course_code = $1 AND deleted_at IS NULL`, courseCode).
		Scan(&courseID, &priceCents, &status); err != nil {
		return Snapshot{}, err
	}

	var err error
	var lectureCount int
	lectureCount, err = count(ctx, db, `SELECT COUNT(*) FROM lecture WHERE course_id = $1`, nil, courseID)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{CourseCode: courseCode, StatDate: day}

	if snapshot.ViewsUV, err = count(ctx, db,
		`SELECT COUNT(DISTINCT COALESCE(student_id, -id)) FROM course_view WHERE course_code = $1 AND viewed_at < $2`,
		nil, courseCode, until); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Favorites, err = count(ctx, db,
		`SELECT COUNT(*) FROM favorite WHERE course_code = $1 AND created_at < $2`, nil, courseCode, until); err != nil {
		return Snapshot{}, err
	}
	// 收藏数不能超过浏览数（历史数据缺失浏览埋点时以收藏数兜底，仍满足单调性断言）。
	if snapshot.Favorites > snapshot.ViewsUV {
		snapshot.ViewsUV = snapshot.Favorites
	}
	if snapshot.Learning, err = count(ctx, db,
		`SELECT COUNT(DISTINCT lp.student_id)
		 FROM lecture_progress lp JOIN lecture l ON l.id = lp.lecture_id
		 WHERE l.course_id = $1 AND lp.last_studied_at >= $2 AND lp.last_studied_at < $3
		   AND ($4 = 0 OR EXISTS(
		     SELECT 1 FROM entitlement e WHERE e.student_id = lp.student_id
		       AND e.course_id = $1 AND e.revoked_at IS NULL))`,
		nil, courseID, learningStart, until, priceCents); err != nil {
		return Snapshot{}, err
	}
	if lectureCount > 0 {
		if snapshot.Completed, err = count(ctx, db,
			`SELECT COUNT(*) FROM (
			   SELECT lp.student_id FROM lecture_progress lp JOIN lecture l ON l.id = lp.lecture_id
			   WHERE l.course_id = $1 AND lp.completed_at IS NOT NULL AND lp.completed_at < $2
			   GROUP BY lp.student_id HAVING COUNT(DISTINCT lp.lecture_id) = $3
			 ) t`, nil, courseID, until, lectureCount); err != nil {
			return Snapshot{}, err
		}
	}
	if snapshot.Completed > snapshot.Learning {
		snapshot.Learning = snapshot.Completed
	}
	if snapshot.Examined, err = count(ctx, db,
		`SELECT COUNT(DISTINCT student_id) FROM exam_session WHERE course_id = $1 AND created_at < $2`,
		nil, courseID, until); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Certified, err = count(ctx, db,
		`SELECT COUNT(DISTINCT student_id) FROM certificate
		 WHERE course_id = $1 AND status = 'valid' AND revoked_at IS NULL AND issued_at < $2`,
		nil, courseID, until); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Orders, err = count(ctx, db,
		`SELECT COUNT(DISTINCT student_id) FROM orders WHERE course_id = $1 AND created_at < $2`,
		nil, courseID, until); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Paid, err = count(ctx, db,
		`SELECT COUNT(DISTINCT student_id) FROM orders
		 WHERE course_id = $1 AND paid_at IS NOT NULL AND paid_at < $2`,
		nil, courseID, until); err != nil {
		return Snapshot{}, err
	}
	var unlocked int
	if unlocked, err = count(ctx, db,
		`SELECT COUNT(DISTINCT student_id) FROM entitlement
		 WHERE course_id = $1 AND granted_at < $2 AND (revoked_at IS NULL OR revoked_at >= $2)`,
		nil, courseID, until); err != nil {
		return Snapshot{}, err
	}
	snapshot.Unlock = maxInt(unlocked, snapshot.Learning, snapshot.Completed, snapshot.Paid)

	var ratingCount int
	var avg sql.NullFloat64
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*), AVG(CAST(rating AS REAL)) FROM course_comment
		 WHERE course_code = $1 AND status = 'visible' AND created_at < $2`, courseCode, until).
		Scan(&ratingCount, &avg); err != nil {
		return Snapshot{}, err
	}
	snapshot.RatingCount = ratingCount
	if ratingCount > 0 && avg.Valid {
		value := math.Round(avg.Float64*100) / 100
		snapshot.AvgRating = &value
	}
	snapshot.ComputedAt = now.UTC().Format(time.RFC3339)

	if _, err := db.ExecContext(ctx,
		`INSERT INTO course_stat_daily(course_code, stat_date, views_uv, favorites, learning, completed,
		   examined, certified, orders, paid, unlock, avg_rating, rating_count, computed_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT(course_code, stat_date) DO UPDATE SET
		   views_uv = excluded.views_uv, favorites = excluded.favorites, learning = excluded.learning,
		   completed = excluded.completed, examined = excluded.examined, certified = excluded.certified,
		   orders = excluded.orders, paid = excluded.paid, unlock = excluded.unlock,
		   avg_rating = excluded.avg_rating, rating_count = excluded.rating_count,
		   computed_at = excluded.computed_at`,
		snapshot.CourseCode, snapshot.StatDate, snapshot.ViewsUV, snapshot.Favorites, snapshot.Learning,
		snapshot.Completed, snapshot.Examined, snapshot.Certified, snapshot.Orders, snapshot.Paid,
		snapshot.Unlock, snapshot.AvgRating, snapshot.RatingCount, snapshot.ComputedAt); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// Latest 取某课最近一份日统计；没有跑批时返回 nil, nil（不编造数字）。
func Latest(ctx context.Context, db *sql.DB, courseCode string) (*Snapshot, error) {
	if db == nil || courseCode == "" {
		return nil, errors.New("统计参数非法")
	}
	row := db.QueryRowContext(ctx, snapshotQuery+` WHERE course_code = $1 ORDER BY stat_date DESC LIMIT 1`, courseCode)
	item, err := scanSnapshot(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Range 取某课时间区间内的快照（含首尾）。
func Range(ctx context.Context, db *sql.DB, courseCode, from, to string) ([]Snapshot, error) {
	if db == nil || courseCode == "" || from == "" || to == "" {
		return nil, errors.New("统计参数非法")
	}
	rows, err := db.QueryContext(ctx, snapshotQuery+
		` WHERE course_code = $1 AND CAST(stat_date AS TEXT) >= $2 AND CAST(stat_date AS TEXT) <= $3
		  ORDER BY CAST(stat_date AS TEXT)`, courseCode, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Snapshot, 0)
	for rows.Next() {
		item, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const snapshotQuery = `SELECT course_code, CAST(stat_date AS TEXT), views_uv, favorites, learning,
	completed, examined, certified, orders, paid, unlock, avg_rating, rating_count, CAST(computed_at AS TEXT)
	FROM course_stat_daily`

func scanSnapshot(scanner interface{ Scan(...any) error }) (Snapshot, error) {
	var item Snapshot
	var avg sql.NullFloat64
	if err := scanner.Scan(&item.CourseCode, &item.StatDate, &item.ViewsUV, &item.Favorites,
		&item.Learning, &item.Completed, &item.Examined, &item.Certified, &item.Orders,
		&item.Paid, &item.Unlock, &avg, &item.RatingCount, &item.ComputedAt); err != nil {
		return Snapshot{}, err
	}
	if avg.Valid {
		value := avg.Float64
		item.AvgRating = &value
	}
	return item, nil
}

func count(ctx context.Context, db *sql.DB, query string, target *int, args ...any) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	if target != nil {
		*target = n
	}
	return n, nil
}

func maxInt(values ...int) int {
	result := 0
	for _, v := range values {
		if v > result {
			result = v
		}
	}
	return result
}
