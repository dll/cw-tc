package exam

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStartSessionStoresDeviceAndTakeover(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, err := BuildPaper(ctx, database, 1, "L1", 0)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := StartSessionWithDevice(ctx, database, 1, 1, "L1", paper, "dev-a")
	if err != nil {
		t.Fatal(err)
	}
	if matched, err := DeviceMatches(ctx, database, sid, "dev-a"); err != nil || !matched {
		t.Fatalf("当前设备应匹配: matched=%v err=%v", matched, err)
	}
	if matched, err := DeviceMatches(ctx, database, sid, "dev-b"); err != nil || matched {
		t.Fatalf("其他设备不应匹配: matched=%v err=%v", matched, err)
	}
	state, err := Takeover(ctx, database, sid, "dev-b", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := state.Answers["_device_key"]; leaked {
		t.Fatal("接管响应不得下发设备标识")
	}
	if matched, err := DeviceMatches(ctx, database, sid, "dev-b"); err != nil || !matched {
		t.Fatalf("接管后新设备应匹配: matched=%v err=%v", matched, err)
	}
	if matched, _ := DeviceMatches(ctx, database, sid, "dev-a"); matched {
		t.Fatal("接管后旧设备不应再匹配")
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM exam_anomaly WHERE exam_id = $1 AND kind = 'device_change'`, sid).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("应记录 1 条 device_change 异常，实际 %d", count)
	}
	var raw string
	_ = database.QueryRow(`SELECT answers FROM exam_session WHERE id = $1`, sid).Scan(&raw)
	if !strings.Contains(raw, "dev-b") {
		t.Fatalf("答案信封里应写入新设备标识: %s", raw)
	}
}

func TestTakeoverValidationErrors(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, err := StartSessionWithDevice(ctx, database, 1, 1, "L1", paper, "dev-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Takeover(ctx, database, sid, "", time.Now().UTC()); !errors.Is(err, ErrDeviceKeyRequired) {
		t.Fatalf("缺设备标识应拒绝，实际 %v", err)
	}
	if _, err := Takeover(ctx, database, sid, "dev-b", time.Now().UTC().Add(2*time.Hour)); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("过期会话接管应拒绝，实际 %v", err)
	}
	if _, err := Takeover(ctx, database, 9999, "dev-b", time.Now().UTC()); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("不存在会话接管应拒绝，实际 %v", err)
	}
	if err := SetDeviceKey(ctx, database, sid, ""); !errors.Is(err, ErrDeviceKeyRequired) {
		t.Fatalf("空设备标识写入应拒绝，实际 %v", err)
	}
	if err := SetDeviceKey(ctx, database, 9999, "x"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("不存在会话写设备应拒绝，实际 %v", err)
	}
	if _, err := DeviceMatches(ctx, database, 9999, "x"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("不存在会话校验设备应报错，实际 %v", err)
	}
	if _, err := database.Exec(`UPDATE exam_session SET status = 'graded' WHERE id = $1`, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := Takeover(ctx, database, sid, "dev-b", time.Now().UTC()); !errors.Is(err, ErrSessionNotOngoing) {
		t.Fatalf("已交卷会话接管应拒绝，实际 %v", err)
	}
}

func TestDeviceMatchesAllowsLegacySession(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	grantEntitlement(t, database, 1, 1)
	paper, _ := BuildPaper(ctx, database, 1, "L1", 0)
	sid, err := StartSession(ctx, database, 1, 1, "L1", paper)
	if err != nil {
		t.Fatal(err)
	}
	if matched, err := DeviceMatches(ctx, database, sid, "any-device"); err != nil || !matched {
		t.Fatalf("无设备标识的历史会话应放行: matched=%v err=%v", matched, err)
	}
}
