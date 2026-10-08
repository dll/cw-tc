package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/staff"
)

// seedAdmin 实现 `cw-tc-server seed-admin` 的核心逻辑（可测试）。
//
// 读取（不写死默认密码，密码缺失即拒绝）：
//
//	CWTC_ADMIN_ACCOUNT（默认 admin）· CWTC_ADMIN_NAME（默认 超级管理员）· CWTC_ADMIN_PASSWORD（必填）
//
// 只连数据库（轻量配置，不校验身份证等强制密钥 —— 种子命令无需它们）。
// 返回 (account, id, created, err)。
func seedAdmin(get config.Getenv) (string, int64, bool, error) {
	account := strings.TrimSpace(get(config.EnvAdminAccount))
	if account == "" {
		account = "admin"
	}
	name := strings.TrimSpace(get(config.EnvAdminName))
	password := get(config.EnvAdminPassword)
	if password == "" {
		return "", 0, false, fmt.Errorf("初始超管密码未配置（%s）：密码绝不写死默认值", config.EnvAdminPassword)
	}

	cfg := &config.Config{
		DeployEnv: strings.ToLower(strings.TrimSpace(get(config.EnvDeployEnv))),
		DBDriver:  strings.ToLower(strings.TrimSpace(get(config.EnvDBDriver))),
		DBDSN:     strings.TrimSpace(get(config.EnvDBDSN)),
	}
	if cfg.DeployEnv == "" {
		cfg.DeployEnv = "development"
	}
	if cfg.DBDriver == "" {
		cfg.DBDriver = "sqlite"
	}
	database, closeDB, err := openDatabase(context.Background(), cfg)
	if err != nil || database == nil {
		return "", 0, false, fmt.Errorf("数据库初始化失败: %w", err)
	}
	defer closeDB()

	id, created, err := staff.SeedAdmin(context.Background(), database, account, name, password)
	if err != nil {
		return "", 0, false, fmt.Errorf("创建初始超管失败: %w", err)
	}
	return account, id, created, nil
}

// runSeedAdmin 执行 seed-admin 子命令；错误通过返回值表达（由 main 决定退出码）。
func runSeedAdmin() error {
	account, id, created, err := seedAdmin(config.OSGetenv)
	if err != nil {
		slog.Error("种子命令失败", "err", err)
		return err
	}
	if created {
		slog.Info("初始超管已创建（首登强制改密）", "account", account, "role", "admin_super", "id", id)
	} else {
		slog.Info("初始超管已存在，幂等跳过", "account", account, "id", id)
	}
	return nil
}
