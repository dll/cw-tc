package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/stats"
)

// runStatsBatch 实现 `cw-tc-server stats-batch [YYYY-MM-DD]`（T+1 跑批）。
//
// 不传日期时跑“昨天”（Asia/Shanghai）。同一天重复跑结果一致（幂等 upsert）。
func runStatsBatch(args []string) error {
	cfg := &config.Config{
		DeployEnv: strings.TrimSpace(os.Getenv(config.EnvDeployEnv)),
		DBDriver:  strings.TrimSpace(os.Getenv(config.EnvDBDriver)),
		DBDSN:     strings.TrimSpace(os.Getenv(config.EnvDBDSN)),
	}
	if cfg.DeployEnv == "" {
		cfg.DeployEnv = "development"
	}
	if cfg.DBDriver == "" {
		cfg.DBDriver = "sqlite"
	}
	database, closeDB, err := openDatabase(context.Background(), cfg)
	if err != nil {
		return err
	}
	defer closeDB()

	now := time.Now().UTC()
	day := now.Add(-24 * time.Hour)
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		parsed, ok := stats.ParseDate(strings.TrimSpace(args[0]))
		if !ok {
			slog.Error("日期格式非法，应为 YYYY-MM-DD", "value", args[0])
			return errInvalidDate(args[0])
		}
		day = parsed
	}
	items, err := stats.RecomputeAll(context.Background(), database, day, now)
	if err != nil {
		return err
	}
	slog.Info("课程统计跑批完成", "stat_date", stats.DateOf(day), "courses", len(items))
	return nil
}

type invalidDate string

func (e invalidDate) Error() string { return "日期格式非法：" + string(e) }

func errInvalidDate(raw string) error { return invalidDate(raw) }
