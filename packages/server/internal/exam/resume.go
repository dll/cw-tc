package exam

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// SaveAnswer 按题即时持久化答案（断线不丢，PRD §7.7 C-7 第 3 条）。仅 ongoing 会话可写。
func SaveAnswer(ctx context.Context, db *sql.DB, sessionID int64, qno, answer string) error {
	var status string
	var raw sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT status, answers FROM exam_session WHERE id = $1`, sessionID).Scan(&status, &raw); err != nil {
		return ErrSessionNotFound
	}
	if status != "ongoing" {
		return ErrSessionNotOngoing
	}
	answers := map[string]string{}
	if raw.Valid {
		_ = json.Unmarshal([]byte(raw.String), &answers)
	}
	answers[qno] = answer
	body, _ := json.Marshal(answers)
	_, err := db.ExecContext(ctx, `UPDATE exam_session SET answers = $1 WHERE id = $2 AND status = 'ongoing'`, string(body), sessionID)
	return err
}

// ResumeState 断线重连恢复的状态。
type ResumeState struct {
	Answers       map[string]string `json:"answers"`
	Deadline      time.Time         `json:"deadline"`
	RemainingSecs int64             `json:"remaining_secs"`
	Expired       bool              `json:"expired"`
}

// Resume 断线重连：返回已答内容与剩余时间。
// ⚠️ 剩余时间一律以**服务端时钟**计算（server_deadline_at - 服务端 now），客户端时钟不可信（铁律 15）。
func Resume(ctx context.Context, db *sql.DB, sessionID int64, now time.Time) (ResumeState, error) {
	var status string
	var raw sql.NullString
	var deadlineRaw string
	if err := db.QueryRowContext(ctx, `SELECT status, answers, server_deadline_at FROM exam_session WHERE id = $1`, sessionID).
		Scan(&status, &raw, &deadlineRaw); err != nil {
		return ResumeState{}, ErrSessionNotFound
	}
	if status != "ongoing" {
		return ResumeState{}, ErrSessionNotOngoing
	}
	deadline, err := time.Parse(time.RFC3339, deadlineRaw)
	if err != nil {
		return ResumeState{}, err
	}
	answers := map[string]string{}
	if raw.Valid {
		_ = json.Unmarshal([]byte(raw.String), &answers)
	}
	remaining := int64(deadline.Sub(now).Seconds())
	return ResumeState{
		Answers:       answers,
		Deadline:      deadline,
		RemainingSecs: remaining,
		Expired:       remaining <= 0,
	}, nil
}
