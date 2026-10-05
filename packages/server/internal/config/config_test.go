package config

import (
	"strings"
	"testing"
)

// validEnv 返回一套通过校验的环境变量（强制密钥齐备）。
func validEnv() map[string]string {
	return map[string]string{
		EnvIDCardEncKey:  strings.Repeat("a", IDCardEncKeyBytes),
		EnvIDCardHMACKey: strings.Repeat("b", MinHMACKeyBytes),
		EnvSessionSecret: strings.Repeat("c", MinHMACKeyBytes),
	}
}

func getter(m map[string]string) Getenv {
	return func(k string) string { return m[k] }
}

func TestLoadFrom_OK(t *testing.T) {
	c, err := LoadFrom(getter(validEnv()))
	if err != nil {
		t.Fatalf("期望加载成功，实际报错: %v", err)
	}
	// 默认监听回环，避免通配绑定（对照 ccit 约定）
	if c.Host != "127.0.0.1" {
		t.Errorf("默认 Host 应为 127.0.0.1，实际 %q", c.Host)
	}
	if c.Port != "8071" {
		t.Errorf("默认 Port 应为 8071，实际 %q", c.Port)
	}
	if c.Addr() != "127.0.0.1:8071" {
		t.Errorf("Addr 组装错误: %q", c.Addr())
	}
	// 行为开关默认值
	if !c.LogRedactEnabled {
		t.Error("日志脱敏应默认开启 —— 关闭它等于放弃铁律 3")
	}
	if !c.PublicStatusEnabled {
		t.Error("公开状态接口应默认开启")
	}
}

func TestValidate_MissingSecrets(t *testing.T) {
	// 三项强制密钥全缺 -> 错误信息必须把三项都列出来（便于一次改完）
	_, err := LoadFrom(getter(map[string]string{}))
	if err == nil {
		t.Fatal("强制密钥缺失时必须报错（服务拒绝启动）")
	}
	for _, want := range []string{EnvIDCardEncKey, EnvIDCardHMACKey, EnvSessionSecret} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息应列出缺失项 %s，实际: %v", want, err)
		}
	}
}

func TestValidate_EncKeyWrongLength(t *testing.T) {
	env := validEnv()
	env[EnvIDCardEncKey] = strings.Repeat("a", 31) // AES-256-GCM 要求 32 字节
	_, err := LoadFrom(getter(env))
	if err == nil {
		t.Fatal("加密密钥长度非 32 字节时必须报错")
	}
	if !strings.Contains(err.Error(), "长度非法") {
		t.Errorf("错误信息应说明长度非法，实际: %v", err)
	}
}

func TestValidate_ShortHMACKey(t *testing.T) {
	env := validEnv()
	env[EnvIDCardHMACKey] = strings.Repeat("b", MinHMACKeyBytes-1)
	if _, err := LoadFrom(getter(env)); err == nil {
		t.Fatal("HMAC 密钥过短时必须报错")
	}
}

// TestFailClosed_OptionalCredentials 验证"未配置即关闭"的安全默认。
func TestFailClosed_OptionalCredentials(t *testing.T) {
	c, err := LoadFrom(getter(validEnv()))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if c.SMSEnabled() {
		t.Error("短信凭据未配置时 SMSEnabled 必须为 false（fail-closed）")
	}
	if c.LLMEnabled() {
		t.Error("模型凭据未配置时 LLMEnabled 必须为 false（fail-closed）")
	}
	if c.WXPayEnabled() {
		t.Error("微信支付凭据未配置时 WXPayEnabled 必须为 false（fail-closed）")
	}
	if c.AlipayEnabled() {
		t.Error("支付宝凭据未配置时 AlipayEnabled 必须为 false（fail-closed）")
	}

	// 只配一半（有 provider 无密钥）仍应视为不可用
	env := validEnv()
	env[EnvSMSProvider] = "tencent"
	c2, err := LoadFrom(getter(env))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if c2.SMSEnabled() {
		t.Error("只配了 provider、缺密钥时 SMSEnabled 仍必须为 false")
	}
}

// TestDatabaseBoundary 验证数据库边界（2026-10-06 明确）：
// 开发/测试 SQLite；生产 PostgreSQL；**生产用 SQLite 一律拒绝**。
func TestDatabaseBoundary(t *testing.T) {
	// 默认（都没设）→ 开发环境 + sqlite
	c, err := LoadFrom(getter(validEnv()))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if c.DeployEnv != "development" {
		t.Errorf("默认 DeployEnv 应为 development，实际 %q", c.DeployEnv)
	}
	if c.DBDriver != "sqlite" {
		t.Errorf("默认 DBDriver 应为 sqlite（开发/测试），实际 %q", c.DBDriver)
	}
	if c.IsProduction() {
		t.Error("默认不应被视为生产环境")
	}

	env := validEnv()

	// 生产 + sqlite → 拒绝启动
	env[EnvDeployEnv] = "production"
	env[EnvDBDriver] = "sqlite"
	if _, err := LoadFrom(getter(env)); err == nil {
		t.Fatal("生产环境使用 SQLite 时必须拒绝启动")
	} else if !strings.Contains(err.Error(), "SQLite") {
		t.Errorf("错误信息应说明生产不得用 SQLite，实际: %v", err)
	}

	// 生产 + 未显式指定驱动 → 默认 sqlite → 同样拒绝（不许靠"忘了设"蒙过去）
	delete(env, EnvDBDriver)
	if _, err := LoadFrom(getter(env)); err == nil {
		t.Fatal("生产环境未显式指定 postgres 时应拒绝（默认 sqlite 不允许）")
	}

	// 生产 + postgres → 通过
	env[EnvDBDriver] = "postgres"
	if _, err := LoadFrom(getter(env)); err != nil {
		t.Fatalf("生产 + postgres 应通过，实际: %v", err)
	}

	// 非法驱动 → 拒绝
	env[EnvDBDriver] = "mysql"
	if _, err := LoadFrom(getter(env)); err == nil {
		t.Fatal("非法驱动取值应报错")
	}
}

func TestBoolDefault(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"true", false, true},
		{"1", false, true},
		{"on", false, true},
		{"false", true, false},
		{"0", true, false},
		{"off", true, false},
		{"", true, true},
		{"   ", false, false},
		{"乱写", true, true},
	}
	for _, tc := range cases {
		if got := boolDefault(tc.in, tc.def); got != tc.want {
			t.Errorf("boolDefault(%q, %v) = %v，期望 %v", tc.in, tc.def, got, tc.want)
		}
	}
}
