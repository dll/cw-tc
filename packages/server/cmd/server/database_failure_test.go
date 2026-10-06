package main

import (
	"context"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestOpenDatabaseRejectsEmptyMigrationConfiguration(t *testing.T) {
	cfg := &config.Config{DeployEnv: "development", DBDriver: "sqlite", DBDSN: "file:invalid?mode=ro"}
	if _, _, err := openDatabase(context.Background(), cfg); err == nil {
		t.Fatal("不可写 SQLite 数据源执行迁移必须失败")
	}
}
