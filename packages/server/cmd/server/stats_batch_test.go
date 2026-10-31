package main

import (
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestRunStatsBatchWithSQLiteFile(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "stats-batch.db")
	t.Setenv(config.EnvDeployEnv, "development")
	t.Setenv(config.EnvDBDriver, "sqlite")
	t.Setenv(config.EnvDBDSN, dsn)

	if err := runStatsBatch([]string{"2026-10-30"}); err != nil {
		t.Fatalf("跑批失败: %v", err)
	}
	// 同一天重复跑必须幂等（不报错、不重复建表）。
	if err := runStatsBatch([]string{"2026-10-30"}); err != nil {
		t.Fatalf("重复跑批应幂等: %v", err)
	}
}

func TestRunStatsBatchRejectsBadDate(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "stats-batch-bad.db")
	t.Setenv(config.EnvDeployEnv, "development")
	t.Setenv(config.EnvDBDriver, "sqlite")
	t.Setenv(config.EnvDBDSN, dsn)
	if err := runStatsBatch([]string{"2026/10/30"}); err == nil {
		t.Fatal("非法日期应报错")
	}
	if err := errInvalidDate("x"); err == nil || err.Error() == "" {
		t.Fatal("错误类型应可用于日志")
	}
}

func TestRunStatsBatchRejectsProductionSQLite(t *testing.T) {
	t.Setenv(config.EnvDeployEnv, "production")
	t.Setenv(config.EnvDBDriver, "sqlite")
	t.Setenv(config.EnvDBDSN, filepath.Join(t.TempDir(), "prod.db"))
	if err := runStatsBatch(nil); err == nil {
		t.Fatal("生产环境使用 SQLite 必须拒绝")
	}
}
