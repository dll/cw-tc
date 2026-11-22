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
	"strings"
	"time"
)

var (
	ErrNoEntitlement  = errors.New("未解锁该课程，无法开考")
	ErrSessionOngoing = errors.New("同档测评已有进行中的会话")
	ErrRetakeCooldown = errors.New("重考冷却期内，暂不可重复开考")
	ErrInvalidLevel   = errors.New("档位非法")
	ErrEmptyPaper     = errors.New("该档位无题可组")
	ErrSessionExpired = errors.New("测评已到时，不能再修改答案")
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
		`SELECT qno, type, stem, options, score FROM question WHERE course_id = $1 AND level = $2 ORDER BY qno`,
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

// StartSession 开考（兼容旧调用方，不绑定设备）。
func StartSession(ctx context.Context, db *sql.DB, studentID, courseID int64, level, paper string) (int64, error) {
	return StartSessionWithDevice(ctx, db, studentID, courseID, level, paper, "")
}

// StartSessionWithDevice 开考：校验权益 + 重考冷却 + 单档单会话，创建 ongoing 会话并绑定当前答题设备。
func StartSessionWithDevice(ctx context.Context, db *sql.DB, studentID, courseID int64, level, paper, deviceKey string) (int64, error) {
	if level != "L1" && level != "L2" && level != "L3" {
		return 0, ErrInvalidLevel
	}
	// 权益校验：已付费解锁（或免费授权）才能开考。
	var ent int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE student_id = $1 AND course_id = $2 AND revoked_at IS NULL`,
		studentID, courseID).Scan(&ent); err != nil {
		return 0, err
	}
	if ent == 0 {
		return 0, ErrNoEntitlement
	}
	now := time.Now().UTC()
	// 先自动结束已到时的 ongoing 会话：否则它会被永久当成“进行中”，
	// 学员再进来只能看到“上次测评已超时”，唯一索引也开不了新会话。
	if _, err := ExpireStale(ctx, db, studentID, courseID, level, now); err != nil {
		return 0, err
	}
	// 重考冷却（PRD §7.7 C-7 · M3-05）：最近一次已交卷会话在冷却期内 → 拒绝。
	// 到时自动交卷的会话跳过冷却：它已经占满整个测评时长，不应再被 24h 冷却二次惩罚。
	var lastSubmitted, lastDeadline sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT submitted_at, server_deadline_at FROM exam_session
		 WHERE student_id = $1 AND course_id = $2 AND level = $3
		   AND status IN ('submitted','graded') AND submitted_at IS NOT NULL
		 ORDER BY submitted_at DESC LIMIT 1`,
		studentID, courseID, level).Scan(&lastSubmitted, &lastDeadline); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if lastSubmitted.Valid {
		if last, err := time.Parse(time.RFC3339, lastSubmitted.String); err == nil && now.Sub(last) < DefaultRetakeCooldown {
			timedOut := false
			if lastDeadline.Valid {
				if deadline, derr := time.Parse(time.RFC3339, lastDeadline.String); derr == nil && !last.Before(deadline) {
					timedOut = true
				}
			}
			if !timedOut {
				return 0, ErrRetakeCooldown
			}
		}
	}
	// 单档单会话（应用层；DB 部分唯一索引由迁移 0019 兜底）。
	var ongoing int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE student_id = $1 AND course_id = $2 AND level = $3 AND status = 'ongoing'`,
		studentID, courseID, level).Scan(&ongoing); err != nil {
		return 0, err
	}
	if ongoing > 0 {
		return 0, ErrSessionOngoing
	}
	// 重考序号 = 该学员该档历史会话数 + 1。
	var history int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE student_id = $1 AND course_id = $2 AND level = $3`,
		studentID, courseID, level).Scan(&history)
	answersJSON := "{}"
	if key := strings.TrimSpace(deviceKey); key != "" {
		body, _ := json.Marshal(map[string]string{deviceAnswerKey: key})
		answersJSON = string(body)
	}
	var sessionID int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO exam_session(student_id, course_id, level, paper, answers, status, started_at, server_deadline_at, retake_seq) VALUES($1,$2,$3,$4,$5,'ongoing',$6,$7,$8) RETURNING id`,
		studentID, courseID, level, paper, answersJSON, now.Format(time.RFC3339), now.Add(DefaultExamDuration).Format(time.RFC3339), history+1).Scan(&sessionID); err != nil {
		return 0, err
	}
	return sessionID, nil
}

// ExpireStale 把已到服务端截止时间的 ongoing 会话自动交卷（按已保存答案判分）。
// 判分失败时退化为标记 submitted，确保单会话槽位被释放，学员不会被永久卡住。
func ExpireStale(ctx context.Context, db *sql.DB, studentID, courseID int64, level string, now time.Time) ([]Result, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id FROM exam_session
		 WHERE student_id = $1 AND course_id = $2 AND level = $3
		   AND status = 'ongoing' AND server_deadline_at IS NOT NULL AND server_deadline_at <= $4
		 ORDER BY id`,
		studentID, courseID, level, now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(ids))
	for _, id := range ids {
		var raw sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT answers FROM exam_session WHERE id = $1`, id).Scan(&raw); err != nil {
			continue
		}
		answers := map[string]string{}
		if raw.Valid {
			_ = json.Unmarshal([]byte(raw.String), &answers)
		}
		res, err := SubmitExam(ctx, db, id, answers)
		if err != nil {
			if errors.Is(err, ErrSessionNotOngoing) {
				continue
			}
			// 判分失败（如历史 paper 损坏）也要释放槽位，避免学员被永久卡住。
			_, _ = db.ExecContext(ctx,
				`UPDATE exam_session SET status = 'submitted', submitted_at = server_deadline_at
				 WHERE id = $1 AND status = 'ongoing'`, id)
			continue
		}
		results = append(results, res)
	}
	return results, nil
}
