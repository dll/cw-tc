package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cw-tc/server/internal/platform/clock"
)

// publicStatusSnapshot 组装供 CCIT 官网 / AOC 读取的**只读**可用性快照。
//
// 硬约束（PRD v2 §7.12 K-2）：
//   - **只聚合计数**，绝不读姓名 / 身份证号（含脱敏）/ 手机号 / 订单金额明细；
//   - **未接数据源**（`s.dbConn == nil`）时如实返回 `unavailable` + `null` 计数
//     —— `null` 表达"未知"，`0` 会被误读为"真的没有课程"，属铁律二禁止的编数；
//   - 查库失败同样回退 `unavailable`，**不得伪造健康**。
func (s *Server) publicStatusSnapshot(ctx context.Context) gin.H {
	observedAt := clock.From(ctx).Now().Format(time.RFC3339)

	// 未接数据源：保持原语义（null + not_wired）。
	if s.dbConn == nil {
		return gin.H{
			"source":     "cw-tc",
			"observedAt": observedAt,
			"status":     statusUnavailable,
			"dataSource": "not_wired",
			"courses": gin.H{
				"total":     nil,
				"published": nil,
				"byMajor":   nil,
			},
			"certificates": gin.H{
				"issued":  nil,
				"revoked": nil,
			},
			"learners": gin.H{
				"registered": nil,
			},
		}
	}

	var total, published, issued, revoked, registered int64
	queries := []struct {
		sql  string
		dest *int64
	}{
		{`SELECT COUNT(*) FROM course`, &total},
		{`SELECT COUNT(*) FROM course WHERE status = 'published'`, &published},
		{`SELECT COUNT(*) FROM certificate`, &issued},
		{`SELECT COUNT(*) FROM certificate WHERE revoked_at IS NOT NULL`, &revoked},
		{`SELECT COUNT(*) FROM student`, &registered},
	}
	for _, q := range queries {
		if err := s.dbConn.QueryRowContext(ctx, q.sql).Scan(q.dest); err != nil {
			// 任一聚合失败即视为数据源不可用，如实降级（不编数）。
			return gin.H{
				"source":       "cw-tc",
				"observedAt":   observedAt,
				"status":       statusDegraded,
				"dataSource":   "database",
				"error":        "aggregate_failed",
				"courses":      gin.H{"total": nil, "published": nil, "byMajor": nil},
				"certificates": gin.H{"issued": nil, "revoked": nil},
				"learners":     gin.H{"registered": nil},
			}
		}
	}

	byMajor := make([]gin.H, 0)
	rows, err := s.dbConn.QueryContext(ctx,
		`SELECT c.major_id, COUNT(*) FROM course c WHERE c.status = 'published' GROUP BY c.major_id ORDER BY 2 DESC, 1 ASC`)
	if err != nil {
		return gin.H{
			"source":       "cw-tc",
			"observedAt":   observedAt,
			"status":       statusDegraded,
			"dataSource":   "database",
			"error":        "aggregate_failed",
			"courses":      gin.H{"total": nil, "published": nil, "byMajor": nil},
			"certificates": gin.H{"issued": nil, "revoked": nil},
			"learners":     gin.H{"registered": nil},
		}
	}
	defer rows.Close()
	for rows.Next() {
		var majorID *int64
		var count int64
		if err := rows.Scan(&majorID, &count); err != nil {
			break
		}
		// 只给 major_id 与计数：**不带专业名称**，避免任何"像个人信息的字段"进入公开响应。
		byMajor = append(byMajor, gin.H{"major_id": majorID, "published": count})
	}

	return gin.H{
		"source":     "cw-tc",
		"observedAt": observedAt,
		"status":     statusAvailable,
		"dataSource": "database",
		"courses": gin.H{
			"total":     total,
			"published": published,
			"byMajor":   byMajor,
		},
		"certificates": gin.H{
			"issued":  issued,
			"revoked": revoked,
		},
		"learners": gin.H{
			"registered": registered,
		},
	}
}
