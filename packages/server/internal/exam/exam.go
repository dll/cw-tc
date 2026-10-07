// Package exam 实现测评引擎：组卷、开考、单档单会话（PRD §7.7 C-7 · 铁律 15）。
//
// 铁律：单档单会话 —— 同档测评同时只允许一个进行中会话。
// 应用层在此校验；DB 部分唯一索引 `uq_exam_ongoing` 由迁移 0019 兜底（本批次不含）。
package exam

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNoEntitlement  = errors.New("未解锁该课程，无法开考")
	ErrSessionOngoing = errors.New("同档测评已有进行中的会话")
	ErrRetakeCooldown = errors.New("重考冷却期内，暂不可重复开考")
	ErrInvalidLevel   = errors.New("档位非法")
	ErrEmptyPaper     = errors.New("该档位无题可组")
)

// PaperItem 是组卷后下发给学员的一道题（不含答案/解析/评分要点，铁律：答案绝不下发）。
type PaperItem struct {
	Qno     string   `json:"qno"`
	Type    string   `json:"type"`
	Stem    string   `json:"stem"`
	Options []string `json:"options,omitempty"`
	Score   float64  `json:"score"`
}

// BuildPaper 组卷：按课程 + 档位抽取题目，返回不含答案的题单 JSON（limit<=0 表示全量）。
func BuildPaper(ctx context.Context, db *sql.DB, courseID int64, level string, limit int) (string, error) {
	if level != "L1" && level != "L2" && level != "L3" {
		return "", ErrInvalidLevel
	}
	rows, err := db.QueryContext(ctx,
		`SELECT qno, type, stem, options, score FROM question WHERE course_id = ? AND level = ? ORDER BY qno`,
		courseID, level)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var items []PaperItem
	for rows.Next() {
		var it PaperItem
		var options sql.NullString
		if err := rows.Scan(&it.Qno, &it.Type, &it.Stem, &options, &it.Score); err != nil {
			return "", err
		}
		if options.Valid {
			_ = json.Unmarshal([]byte(options.String), &it.Options)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", ErrEmptyPaper
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	body, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// DefaultExamDuration 测评时长默认值（60 分钟）。上线前迁入 system_config（exam.duration_minutes，铁律 15 不写死）。
const DefaultExamDuration = 60 * time.Minute

// DefaultRetakeCooldown 重考冷却默认值（24 小时）。上线前迁入 system_config（exam.retake_cooldown_hours，铁律 15 不写死）。
const DefaultRetakeCooldown = 24 * time.Hour

// StartSession 开考：校验权益 + 重考冷却 + 单档单会话，再创建 ongoing 会话（含服务端截止时间与重考序号），返回 session id。
func StartSession(ctx context.Context, db *sql.DB, studentID, courseID int64, level, paper string) (int64, error) {
	if level != "L1" && level != "L2" && level != "L3" {
		return 0, ErrInvalidLevel
	}
	// 权益校验：已付费解锁（或免费授权）才能开考。
	var ent int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE student_id = ? AND course_id = ? AND revoked_at IS NULL`,
		studentID, courseID).Scan(&ent); err != nil {
		return 0, err
	}
	if ent == 0 {
		return 0, ErrNoEntitlement
	}
	now := time.Now().UTC()
	// 重考冷却（PRD §7.7 C-7 · M3-05）：最近一次已交卷会话在冷却期内 → 拒绝。
	var lastSubmitted sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(submitted_at) FROM exam_session WHERE student_id = ? AND course_id = ? AND level = ? AND status IN ('submitted','graded')`,
		studentID, courseID, level).Scan(&lastSubmitted); err != nil {
		return 0, err
	}
	if lastSubmitted.Valid {
		if last, err := time.Parse(time.RFC3339, lastSubmitted.String); err == nil && now.Sub(last) < DefaultRetakeCooldown {
			return 0, ErrRetakeCooldown
		}
	}
	// 单档单会话（应用层；DB 部分唯一索引由迁移 0019 兜底）。
	var ongoing int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE student_id = ? AND course_id = ? AND level = ? AND status = 'ongoing'`,
		studentID, courseID, level).Scan(&ongoing); err != nil {
		return 0, err
	}
	if ongoing > 0 {
		return 0, ErrSessionOngoing
	}
	// 重考序号 = 该学员该档历史会话数 + 1。
	var history int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE student_id = ? AND course_id = ? AND level = ?`,
		studentID, courseID, level).Scan(&history)
	res, err := db.ExecContext(ctx,
		`INSERT INTO exam_session(student_id, course_id, level, paper, status, started_at, server_deadline_at, retake_seq) VALUES(?,?,?,?,'ongoing',?,?,?)`,
		studentID, courseID, level, paper, now.Format(time.RFC3339), now.Add(DefaultExamDuration).Format(time.RFC3339), history+1)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
