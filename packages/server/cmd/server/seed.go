package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/staff"
)

// seedResult 是单个种子账号的结果。
type seedResult struct {
	Account string
	Name    string
	Role    string
	ID      int64
	Created bool
}

// seedAdmin 实现 `cw-tc-server seed-admin` 的核心逻辑（可测试）。
//
// 一次种子三个角色（超管 / 运营 / 教师），每个账号密码都不写死默认值：
//
//	CWTC_ADMIN_ACCOUNT/PASSWORD   （admin_super，必填，缺失则失败）
//	CWTC_OPS_ACCOUNT/PASSWORD     （admin_ops，可选，缺密码则跳过）
//	CWTC_TEACHER_ACCOUNT/PASSWORD （teacher，可选，缺密码则跳过）
//
// 只连数据库（轻量配置，不校验身份证等强制密钥 —— 种子命令无需它们）。
// 返回 (结果列表, err)。
func seedAdmin(get config.Getenv) ([]seedResult, error) {
	accounts := []struct {
		acctKey, passKey, nameKey string
		defAcct, defName          string
		role                      staff.Role
		required                  bool
	}{
		{config.EnvAdminAccount, config.EnvAdminPassword, config.EnvAdminName, "admin", "超级管理员", staff.RoleAdminSuper, true},
		{config.EnvOpsAccount, config.EnvOpsPassword, config.EnvOpsName, "ops", "运营管理员", staff.RoleAdminOps, false},
		{config.EnvTeacherAccount, config.EnvTeacherPasswd, config.EnvTeacherName, "teacher", "教师", staff.RoleTeacher, false},
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
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}
	defer closeDB()

	results := make([]seedResult, 0, 3)
	for _, a := range accounts {
		account := strings.TrimSpace(get(a.acctKey))
		if account == "" {
			account = a.defAcct
		}
		name := strings.TrimSpace(get(a.nameKey))
		if name == "" {
			name = a.defName
		}
		password := get(a.passKey)
		if password == "" {
			if a.required {
				return nil, fmt.Errorf("初始超管密码未配置（%s）：密码绝不写死默认值", a.passKey)
			}
			slog.Info("跳过种子账号（密码未配置）", "account", account, "role", string(a.role))
			continue
		}
		id, created, err := staff.SeedStaff(context.Background(), database, account, name, a.role, password)
		if err != nil {
			return nil, fmt.Errorf("创建账号 %s 失败: %w", account, err)
		}
		results = append(results, seedResult{Account: account, Name: name, Role: string(a.role), ID: id, Created: created})
	}
	return results, nil
}

// runSeedAdmin 执行 seed-admin 子命令；错误通过返回值表达（由 main 决定退出码）。
func runSeedAdmin() error {
	results, err := seedAdmin(config.OSGetenv)
	if err != nil {
		slog.Error("种子命令失败", "err", err)
		return err
	}
	for _, r := range results {
		if r.Created {
			slog.Info("账号已创建（首登强制改密）", "account", r.Account, "name", r.Name, "role", r.Role, "id", r.ID)
		} else {
			slog.Info("账号已存在，幂等跳过", "account", r.Account, "role", r.Role, "id", r.ID)
		}
	}
	return nil
}
