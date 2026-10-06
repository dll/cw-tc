package main

import (
	"context"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestOpenDatabaseRejectsNilConfig(t *testing.T) {
	if _, _, err := openDatabase(context.Background(), nil); err == nil {
		t.Fatal("空配置必须拒绝")
	}
}

func TestOpenDatabaseRejectsUnknownDialect(t *testing.T) {
	cfg := &config.Config{DeployEnv: "development", DBDriver: "mysql"}
	if _, _, err := openDatabase(context.Background(), cfg); err == nil {
		t.Fatal("未知数据库方言必须拒绝")
	}
}
