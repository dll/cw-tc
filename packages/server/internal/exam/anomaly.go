package exam

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// 异常信号类型（PRD §7.7 C-7 · 铁律 15：只记信号，不判负）。
type AnomalyKind string

const (
	AnomalyBlur             AnomalyKind = "blur"               // 切屏/离开
	AnomalyTooFast          AnomalyKind = "too_fast"           // 单题秒答
	AnomalySameOptionStreak AnomalyKind = "same_option_streak" // 连击同选项
	AnomalyDeviceChange     AnomalyKind = "device_change"      // 设备突变
	AnomalyIPChange         AnomalyKind = "ip_change"
	AnomalyMultiSession     AnomalyKind = "multi_session"
	AnomalyNoHeartbeat      AnomalyKind = "no_heartbeat"
)

var ErrAnomalyKind = errors.New("异常信号类型非法")

// RecordAnomaly 记录一条异常信号：只写 exam_anomaly（verdict=pending），不改成绩、不判负。
func RecordAnomaly(ctx context.Context, db *sql.DB, sessionID int64, kind AnomalyKind, detail string) error {
	if !validAnomalyKind(kind) {
		return ErrAnomalyKind
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exam_session WHERE id = ?`, sessionID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrSessionNotFound
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO exam_anomaly(exam_id, kind, detail, at, verdict) VALUES(?,?,?,?,'pending')`,
		sessionID, string(kind), detail, time.Now().UTC().Format(time.RFC3339))
	return err
}

func validAnomalyKind(k AnomalyKind) bool {
	switch k {
	case AnomalyBlur, AnomalyTooFast, AnomalySameOptionStreak, AnomalyDeviceChange, AnomalyIPChange, AnomalyMultiSession, AnomalyNoHeartbeat:
		return true
	}
	return false
}
