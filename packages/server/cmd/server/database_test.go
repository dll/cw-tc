package main

import (
	"context"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestOpenDevelopmentDatabaseUsesSQLiteAndAppliesMigrations(t *testing.T) {
	cfg := &config.Config{DeployEnv: "development", DBDriver: "sqlite"}
	database, closeDB, err := openDatabase(context.Background(), cfg)
	if err != nil {
		t.Fatalf("开发数据库打开失败: %v", err)
	}
	defer closeDB()
	var n int
	if err := database.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("迁移记录查询失败: %v", err)
	}
	if n != 17 {
		t.Fatalf("SQLite 迁移数 = %d，期望 17", n)
	}
}

func TestOpenDatabaseRejectsProductionSQLite(t *testing.T) {
	cfg := &config.Config{DeployEnv: "production", DBDriver: "sqlite"}
	if _, _, err := openDatabase(context.Background(), cfg); err == nil {
		t.Fatal("生产环境使用 SQLite 必须拒绝")
	}
}

func TestOpenDatabaseRejectsUnsupportedProductionPostgresLocally(t *testing.T) {
	cfg := &config.Config{DeployEnv: "production", DBDriver: "postgres"}
	if _, _, err := openDatabase(context.Background(), cfg); err == nil {
		t.Fatal("当前未接入可验证 PostgreSQL 驱动时必须显式失败")
	}
}
