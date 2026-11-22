package exam

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// deviceAnswerKey 是 answers JSON 里的保留键：记录当前答题设备标识。
// 用它承载单档单会话的“当前设备”，避免为一次小版本改表；所有对外响应都必须剔除。
const deviceAnswerKey = "_device_key"

var (
	ErrDeviceKeyRequired = errors.New("缺少答题设备标识")
	ErrDeviceMismatch    = errors.New("当前设备不是答题设备，请重新进入并确认接管")
)

// DeviceKeyFromAnswers 读取 answers 里的当前答题设备标识（没有则空串）。
func DeviceKeyFromAnswers(answers map[string]string) string {
	return strings.TrimSpace(answers[deviceAnswerKey])
}

// WithoutDeviceKey 返回剔除保留键后的答案副本，供接口下发给客户端。
func WithoutDeviceKey(answers map[string]string) map[string]string {
	out := make(map[string]string, len(answers))
	for key, value := range answers {
		if key == deviceAnswerKey {
			continue
		}
		out[key] = value
	}
	return out
}

// SetDeviceKey 把当前设备写入会话 answers（保留已有答案）。
func SetDeviceKey(ctx context.Context, db *sql.DB, sessionID int64, deviceKey string) error {
	deviceKey = strings.TrimSpace(deviceKey)
	if deviceKey == "" {
		return ErrDeviceKeyRequired
	}
	var raw sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT answers FROM exam_session WHERE id = $1`, sessionID).Scan(&raw); err != nil {
		return ErrSessionNotFound
	}
	answers := map[string]string{}
	if raw.Valid {
		_ = json.Unmarshal([]byte(raw.String), &answers)
	}
	answers[deviceAnswerKey] = deviceKey
	body, _ := json.Marshal(answers)
	_, err := db.ExecContext(ctx,
		`UPDATE exam_session SET answers = $1 WHERE id = $2 AND status = 'ongoing'`, string(body), sessionID)
	return err
}

// DeviceMatches 校验请求设备是否是当前答题设备。
// 历史会话没有设备标识、或客户端未上报设备标识时放行（兼容旧数据/旧客户端）。
func DeviceMatches(ctx context.Context, db *sql.DB, sessionID int64, deviceKey string) (bool, error) {
	var raw sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT answers FROM exam_session WHERE id = $1`, sessionID).Scan(&raw); err != nil {
		return false, ErrSessionNotFound
	}
	answers := map[string]string{}
	if raw.Valid {
		_ = json.Unmarshal([]byte(raw.String), &answers)
	}
	active := DeviceKeyFromAnswers(answers)
	deviceKey = strings.TrimSpace(deviceKey)
	if active == "" || deviceKey == "" {
		return true, nil
	}
	return active == deviceKey, nil
}

// Takeover 显式换设备接管：写入新设备标识 + 记录 device_change 异常（只标记，不判负）。
func Takeover(ctx context.Context, db *sql.DB, sessionID int64, deviceKey string, now time.Time) (ResumeState, error) {
	deviceKey = strings.TrimSpace(deviceKey)
	if deviceKey == "" {
		return ResumeState{}, ErrDeviceKeyRequired
	}
	var status, deadlineRaw string
	var raw sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT status, answers, server_deadline_at FROM exam_session WHERE id = $1`, sessionID).
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
	if !now.Before(deadline) {
		return ResumeState{}, ErrSessionExpired
	}
	answers := map[string]string{}
	if raw.Valid {
		_ = json.Unmarshal([]byte(raw.String), &answers)
	}
	answers[deviceAnswerKey] = deviceKey
	body, _ := json.Marshal(answers)
	if _, err := db.ExecContext(ctx,
		`UPDATE exam_session SET answers = $1 WHERE id = $2 AND status = 'ongoing'`, string(body), sessionID); err != nil {
		return ResumeState{}, err
	}
	if err := RecordAnomaly(ctx, db, sessionID, AnomalyDeviceChange, "换设备接管"); err != nil {
		return ResumeState{}, err
	}
	return ResumeState{
		Answers:       WithoutDeviceKey(answers),
		Deadline:      deadline,
		RemainingSecs: int64(deadline.Sub(now).Seconds()),
		Expired:       false,
	}, nil
}
