package main

import (
	"path/filepath"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func seedGet(dsn string, admin, ops, teacher bool) config.Getenv {
	return func(k string) string {
		switch k {
		case config.EnvDBDriver:
			return "sqlite"
		case config.EnvDBDSN:
			return dsn
		case config.EnvAdminAccount:
			return "admin"
		case config.EnvAdminPassword:
			if admin {
				return "pw-admin"
			}
		case config.EnvOpsPassword:
			if ops {
				return "pw-ops"
			}
		case config.EnvTeacherPasswd:
			if teacher {
				return "pw-teacher"
			}
		}
		return ""
	}
}

func TestSeedAdminThreeRoles(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	// 三个密码都给 → 三个账号都创建
	results, err := seedAdmin(seedGet(dsn, true, true, true))
	if err != nil {
		t.Fatalf("seedAdmin: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("应创建 3 个账号，实际 %d", len(results))
	}
	roles := map[string]bool{}
	for _, r := range results {
		if !r.Created || r.ID <= 0 {
			t.Fatalf("账号 %s 应 created=true id>0，实际 created=%v id=%d", r.Account, r.Created, r.ID)
		}
		roles[r.Role] = true
	}
	for _, want := range []string{"admin_super", "admin_ops", "teacher"} {
		if !roles[want] {
			t.Fatalf("缺角色 %s（实际 %v）", want, roles)
		}
	}
	// 幂等：再次种子全部跳过
	results2, err := seedAdmin(seedGet(dsn, true, true, true))
	if err != nil {
		t.Fatalf("seedAdmin 幂等: %v", err)
	}
	for _, r := range results2 {
		if r.Created {
			t.Fatalf("二次种子应幂等跳过，实际 %s created=true", r.Account)
		}
	}
}

func TestSeedAdminOnlyRequired(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	// 只给超管密码 → 只创建超管，运营/教师跳过
	results, err := seedAdmin(seedGet(dsn, true, false, false))
	if err != nil {
		t.Fatalf("seedAdmin: %v", err)
	}
	if len(results) != 1 || results[0].Role != "admin_super" {
		t.Fatalf("只给超管密码应仅创建超管，实际 %+v", results)
	}
}

func TestSeedAdminMissingAdminPassword(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	if _, err := seedAdmin(seedGet(dsn, false, true, true)); err == nil {
		t.Fatalf("缺超管密码应报错")
	}
}

func TestSeedAdminBadDriver(t *testing.T) {
	get := func(k string) string {
		if k == config.EnvDBDriver {
			return "oracle"
		}
		if k == config.EnvAdminPassword {
			return "pw"
		}
		return "x"
	}
	if _, err := seedAdmin(get); err == nil {
		t.Fatalf("非法驱动应报错")
	}
}

func TestRunSeedAdminDirect(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "seed.db")
	t.Setenv(config.EnvDBDriver, "sqlite")
	t.Setenv(config.EnvDBDSN, dsn)
	t.Setenv(config.EnvAdminAccount, "admin")
	t.Setenv(config.EnvAdminPassword, "pw-12345")
	t.Setenv(config.EnvOpsPassword, "pw-ops")
	t.Setenv(config.EnvTeacherPasswd, "pw-teacher")

	// 成功路径
	if err := runSeedAdmin(); err != nil {
		t.Fatalf("runSeedAdmin: %v", err)
	}
	// 幂等路径
	if err := runSeedAdmin(); err != nil {
		t.Fatalf("runSeedAdmin 幂等: %v", err)
	}
	// 错误路径（缺超管密码 → 返回错误）
	t.Setenv(config.EnvAdminPassword, "")
	if err := runSeedAdmin(); err == nil {
		t.Fatalf("缺超管密码应返回错误")
	}
}
