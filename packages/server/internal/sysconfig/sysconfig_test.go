package sysconfig_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/sysconfig"
	"github.com/cw-tc/server/migrations"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO staff(id, account, name, role, password_hash, created_source) VALUES(1,'boss','超管','admin_super',X'01','seed')`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return database
}

func TestListAndDefaults(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	store := sysconfig.New(database)

	items, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("首批应有 3 个业务参数，实际 %d", len(items))
	}
	if got := store.Int(context.Background(), sysconfig.KeyStatsDisplayThreshold, 99); got != 10 {
		t.Fatalf("stats.display_threshold 默认应为 10，实际 %d", got)
	}
	if got := store.Int(context.Background(), sysconfig.KeyOfflineValidDays, 99); got != 30 {
		t.Fatalf("offline.valid_days 默认应为 30，实际 %d", got)
	}
}

func TestSetEnforcesBoundsAndWhitelist(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	store := sysconfig.New(database)
	ctx := context.Background()

	item, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if item.Value != 5 {
		t.Fatalf("写入值异常: %+v", item)
	}
	if got := store.Int(ctx, sysconfig.KeyStatsDisplayThreshold, 10); got != 5 {
		t.Fatalf("修改后应立即生效，实际 %d", got)
	}
	// 下限保护：< 3 一律拒绝（0 / 1 / 2 都不行）。
	for _, bad := range []int{0, 1, 2} {
		if _, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, bad, 1); !errors.Is(err, sysconfig.ErrBadValue) {
			t.Fatalf("阈值 %d 应被拒，实际 %v", bad, err)
		}
	}
	if _, err := store.Set(ctx, "secret.api_key", "x", 1); !errors.Is(err, sysconfig.ErrUnknownKey) {
		t.Fatalf("未登记键应被拒，实际 %v", err)
	}
	if _, err := store.Set(ctx, sysconfig.KeyCommentDisplayThreshold, "abc", 1); !errors.Is(err, sysconfig.ErrBadValue) {
		t.Fatalf("非整数应被拒，实际 %v", err)
	}
}

func TestEnvironmentOverridesTable(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	store := sysconfig.New(database)
	ctx := context.Background()
	if _, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, 5, 1); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CWTC_STATS_DISPLAY_THRESHOLD", "7")
	if got := store.Int(ctx, sysconfig.KeyStatsDisplayThreshold, 10); got != 7 {
		t.Fatalf("环境变量优先级最高，实际 %d", got)
	}
}

func TestGetAndNonIntValues(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	ctx := context.Background()
	store := sysconfig.New(database)
	if _, err := store.Set(ctx, sysconfig.KeyOfflineValidDays, 999, 1); !errors.Is(err, sysconfig.ErrBadValue) {
		t.Fatalf("超过上限应被拒: %v", err)
	}
	rows := []string{
		`INSERT INTO system_config(key, value, value_type, description) VALUES('demo.flag','true','bool','演示布尔')`,
		`INSERT INTO system_config(key, value, value_type, description) VALUES('demo.text','"hello"','string','演示字符串')`,
		`INSERT INTO system_config(key, value, value_type, description) VALUES('demo.json','{"a":1}','json','演示 JSON')`,
	}
	for _, row := range rows {
		if _, err := database.Exec(row); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.List(ctx)
	if err != nil || len(items) != 6 {
		t.Fatalf("列表应包含 6 项: %d %v", len(items), err)
	}
	flag, err := store.Get(ctx, "demo.flag")
	if err != nil || flag.Value != true {
		t.Fatalf("布尔值解析异常: %+v %v", flag, err)
	}
	text, err := store.Get(ctx, "demo.text")
	if err != nil || text.Value != "hello" {
		t.Fatalf("字符串解析异常: %+v %v", text, err)
	}
	if _, err := store.Get(ctx, "missing.key"); !errors.Is(err, sysconfig.ErrUnknownKey) {
		t.Fatalf("不存在键应报 ErrUnknownKey: %v", err)
	}
	if _, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, true, 1); !errors.Is(err, sysconfig.ErrBadValue) {
		t.Fatalf("非整数类型应拒绝: %v", err)
	}
	if _, err := sysconfig.New(nil).List(ctx); err == nil {
		t.Fatal("数据库为空时应报错")
	}
}

func TestInvalidEnvironmentValueFallsBack(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	t.Setenv("CWTC_STATS_DISPLAY_THRESHOLD", "not-a-number")
	if got := sysconfig.New(database).Int(context.Background(), sysconfig.KeyStatsDisplayThreshold, 10); got != 10 {
		t.Fatalf("非法环境变量应回退默认值，实际 %d", got)
	}
	if !sysconfig.KnownKey(sysconfig.KeyOfflineValidDays) || sysconfig.KnownKey("secret.api_key") {
		t.Fatal("配置键白名单异常")
	}
	if !sysconfig.SecretLike("llm.api_key") || sysconfig.SecretLike(sysconfig.KeyStatsDisplayThreshold) {
		t.Fatal("密钥特征判断异常")
	}
}

func TestCoerceIntVariants(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	ctx := context.Background()
	store := sysconfig.New(database)
	if item, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, int64(6), 1); err != nil || item.Value != 6 {
		t.Fatalf("int64 写入失败: %+v %v", item, err)
	}
	if item, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, "7", 1); err != nil || item.Value != 7 {
		t.Fatalf("字符串数字写入失败: %+v %v", item, err)
	}
	if _, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, float64(5.5), 1); !errors.Is(err, sysconfig.ErrBadValue) {
		t.Fatalf("小数应被拒: %v", err)
	}
	if _, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, nil, 1); !errors.Is(err, sysconfig.ErrBadValue) {
		t.Fatalf("nil 应被拒: %v", err)
	}
	if _, err := store.Set(ctx, sysconfig.KeyStatsDisplayThreshold, 8, 0); !errors.Is(err, sysconfig.ErrUnknownKey) {
		t.Fatalf("缺少修改人应被拒: %v", err)
	}
	if _, err := sysconfig.New(nil).Set(ctx, sysconfig.KeyStatsDisplayThreshold, 5, 1); err == nil {
		t.Fatal("空库写入应报错")
	}
	// 直接写入带引号的 JSON 数字，Int 也应能解析。
	if _, err := database.Exec(`UPDATE system_config SET value='"5"' WHERE key='stats.display_threshold'`); err != nil {
		t.Fatal(err)
	}
	if got := store.Int(ctx, sysconfig.KeyStatsDisplayThreshold, 10); got != 5 {
		t.Fatalf("带引号数字应解析为 5，实际 %d", got)
	}
	// 坏值回退默认。
	if _, err := database.Exec(`UPDATE system_config SET value='abc' WHERE key='stats.display_threshold'`); err != nil {
		t.Fatal(err)
	}
	if got := store.Int(ctx, sysconfig.KeyStatsDisplayThreshold, 10); got != 10 {
		t.Fatalf("坏值应回退默认值，实际 %d", got)
	}
	// 空库 Int 直接回退。
	if got := sysconfig.New(nil).Int(ctx, sysconfig.KeyStatsDisplayThreshold, 9); got != 9 {
		t.Fatalf("空库应回退默认值，实际 %d", got)
	}
}

func TestOfflineDaysEnvOverride(t *testing.T) {
	database := testDB(t)
	defer database.Close()
	t.Setenv("CWTC_OFFLINE_VALID_DAYS", "45")
	if got := sysconfig.New(database).Int(context.Background(), sysconfig.KeyOfflineValidDays, 30); got != 45 {
		t.Fatalf("环境变量应覆盖表值，实际 %d", got)
	}
}