package main

import (
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestSeedAdminFlow(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	get := func(k string) string {
		switch k {
		case config.EnvDBDriver:
			return "sqlite"
		case config.EnvDBDSN:
			return dsn
		case config.EnvAdminAccount:
			return "boss"
		case config.EnvAdminPassword:
			return "初始密码-1"
		case config.EnvAdminName:
			return "超级管理员"
		default:
			return ""
		}
	}

	// 首次创建
	account, id, created, err := seedAdmin(get)
	if err != nil || !created || account != "boss" || id <= 0 {
		t.Fatalf("首次种子失败: account=%s id=%d created=%v err=%v", account, id, created, err)
	}
	// 幂等：再次种子跳过
	_, id2, created2, err := seedAdmin(get)
	if err != nil || created2 || id2 != id {
		t.Fatalf("二次种子应幂等跳过: created=%v id2=%d(想 %d) err=%v", created2, id2, id, err)
	}
}

func TestSeedAdminMissingPassword(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	get := func(k string) string {
		switch k {
		case config.EnvDBDriver:
			return "sqlite"
		case config.EnvDBDSN:
			return dsn
		default:
			return "" // 无密码
		}
	}
	if _, _, _, err := seedAdmin(get); err == nil {
		t.Fatalf("缺密码应报错")
	}
}

func TestSeedAdminBadDriver(t *testing.T) {
	get := func(k string) string {
		switch k {
		case config.EnvDBDriver:
			return "oracle" // 非法驱动
		default:
			return "x"
		}
	}
	if _, _, _, err := seedAdmin(get); err == nil {
		t.Fatalf("非法驱动应报错")
	}
}

func TestRunSeedAdminDirect(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	t.Setenv(config.EnvDBDriver, "sqlite")
	t.Setenv(config.EnvDBDSN, dsn)
	t.Setenv(config.EnvAdminAccount, "admin")
	t.Setenv(config.EnvAdminPassword, "pw-12345")

	// 成功路径（created 分支）
	if err := runSeedAdmin(); err != nil {
		t.Fatalf("runSeedAdmin: %v", err)
	}
	// 幂等路径（已存在分支）
	if err := runSeedAdmin(); err != nil {
		t.Fatalf("runSeedAdmin 幂等: %v", err)
	}
	// 错误路径（缺密码 → 返回错误，不再 os.Exit）
	t.Setenv(config.EnvAdminPassword, "")
	if err := runSeedAdmin(); err == nil {
		t.Fatalf("缺密码应返回错误")
	}
}
