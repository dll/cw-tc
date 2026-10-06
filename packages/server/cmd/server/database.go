package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
)

func openDatabase(ctx context.Context, cfg *config.Config) (*sql.DB, func(), error) {
	if cfg == nil {
		return nil, func() {}, fmt.Errorf("数据库配置为空")
	}
	if cfg.IsProduction() && cfg.DBDriver == string(db.DialectSQLite) {
		return nil, func() {}, fmt.Errorf("生产环境不得使用 SQLite")
	}
	dialect, err := db.ParseDialect(cfg.DBDriver)
	if err != nil {
		return nil, func() {}, err
	}
	database, err := db.Open(ctx, dialect, cfg.DBDSN)
	if err != nil {
		return nil, func() {}, err
	}
	if err := db.ApplyMigrations(ctx, database, migrations.FS, dialect); err != nil {
		_ = database.Close()
		return nil, func() {}, err
	}
	return database, func() { _ = database.Close() }, nil
}
