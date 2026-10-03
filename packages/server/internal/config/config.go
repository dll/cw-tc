// Package config 从环境变量加载服务配置。
//
// 安全默认（PRD v2 §12.2，沿用 ccit 约定）：
//   - 强制密钥（身份证加密密钥 / HMAC 密钥 / 会话密钥）缺失或过短 —— 服务拒绝启动，
//     绝不退回"明文暂存"这类降级路径；
//   - 其余凭据（短信 / 支付 / 大模型）未配置 —— 对应功能 fail-closed 不启用。
//
// 环境变量名与 deploy/.env.example 保持一一对应。
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// 环境变量名常量：与 deploy/.env.example 同步维护。
const (
	EnvServerPort = "SERVER_PORT"
	EnvHost       = "CWTC_HOST"

	// 强制密钥（缺失则拒绝启动）
	EnvIDCardEncKey  = "CWTC_ID_CARD_ENC_KEY"
	EnvIDCardHMACKey = "CWTC_ID_CARD_HMAC_KEY"
	EnvSessionSecret = "CWTC_SESSION_SECRET"

	// 可选凭据（未配置则该功能关闭）
	EnvSMSProvider   = "CWTC_SMS_PROVIDER"
	EnvSMSSecretID   = "CWTC_SMS_SECRET_ID"
	EnvSMSSecretKey  = "CWTC_SMS_SECRET_KEY"
	EnvLLMProvider   = "CWTC_LLM_PROVIDER"
	EnvLLMAPIKey     = "CWTC_LLM_API_KEY"
	EnvWXPayMchID    = "CWTC_WXPAY_MCH_ID"
	EnvWXPayAPIv3Key = "CWTC_WXPAY_API_V3_KEY"
	EnvAlipayAppID   = "CWTC_ALIPAY_APP_ID"
	EnvAlipayPrivKey = "CWTC_ALIPAY_PRIVATE_KEY_PATH"

	// 行为开关
	EnvPublicStatusEnabled = "CWTC_PUBLIC_STATUS_ENABLED"
	EnvLogRedactSensitive  = "CWTC_LOG_REDACT_SENSITIVE"
)

// IDCardEncKeyBytes 是 AES-256-GCM 要求的密钥长度（字节）。
const IDCardEncKeyBytes = 32

// MinHMACKeyBytes 是 HMAC-SHA256 / 会话签名密钥的最小长度（字节）。
const MinHMACKeyBytes = 32

// Config 为服务运行时配置。凭据字段只在本包内流转，
// 由网关层（支付 / AI / 短信）消费；业务层不得直接读取原文。
type Config struct {
	Host string
	Port string

	// 强制密钥
	IDCardEncKey  string
	IDCardHMACKey string
	SessionSecret string

	// 可选凭据
	SMSProvider   string
	SMSSecretID   string
	SMSSecretKey  string
	LLMProvider   string
	LLMAPIKey     string
	WXPayMchID    string
	WXPayAPIv3Key string
	AlipayAppID   string
	AlipayPrivKey string

	PublicStatusEnabled bool
	LogRedactEnabled    bool
}

// Getenv 抽象环境读取，便于测试注入。
type Getenv func(string) string

// OSGetenv 是生产环境使用的 Getenv 实现。
func OSGetenv(key string) string { return os.Getenv(key) }

// Load 使用进程环境变量加载配置。
func Load() (*Config, error) { return LoadFrom(OSGetenv) }

// LoadFrom 按给定取值函数加载配置并校验。
// 返回的 error 会**列出全部**缺失/过短的强制密钥，便于部署时一次改完。
func LoadFrom(get Getenv) (*Config, error) {
	c := &Config{
		Host: strings.TrimSpace(get(EnvHost)),
		Port: strings.TrimSpace(get(EnvServerPort)),

		IDCardEncKey:  strings.TrimSpace(get(EnvIDCardEncKey)),
		IDCardHMACKey: strings.TrimSpace(get(EnvIDCardHMACKey)),
		SessionSecret: strings.TrimSpace(get(EnvSessionSecret)),

		SMSProvider:   strings.TrimSpace(get(EnvSMSProvider)),
		SMSSecretID:   strings.TrimSpace(get(EnvSMSSecretID)),
		SMSSecretKey:  strings.TrimSpace(get(EnvSMSSecretKey)),
		LLMProvider:   strings.TrimSpace(get(EnvLLMProvider)),
		LLMAPIKey:     strings.TrimSpace(get(EnvLLMAPIKey)),
		WXPayMchID:    strings.TrimSpace(get(EnvWXPayMchID)),
		WXPayAPIv3Key: strings.TrimSpace(get(EnvWXPayAPIv3Key)),
		AlipayAppID:   strings.TrimSpace(get(EnvAlipayAppID)),
		AlipayPrivKey: strings.TrimSpace(get(EnvAlipayPrivKey)),

		// 默认值：监听本机回环，避免通配绑定被公网直连伪造 XFF（对照 ccit 约定）。
		PublicStatusEnabled: boolDefault(get(EnvPublicStatusEnabled), true),
		LogRedactEnabled:    boolDefault(get(EnvLogRedactSensitive), true),
	}
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == "" {
		c.Port = "8071"
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate 校验强制密钥。缺失或过短一律报错（服务拒绝启动）。
func (c *Config) Validate() error {
	var problems []string

	switch {
	case c.IDCardEncKey == "":
		problems = append(problems, fmt.Sprintf("%s 未配置（身份证号加密密钥，缺失将无法安全存储）", EnvIDCardEncKey))
	case len(c.IDCardEncKey) != IDCardEncKeyBytes:
		problems = append(problems,
			fmt.Sprintf("%s 长度非法：需 %d 字节，实际 %d 字节", EnvIDCardEncKey, IDCardEncKeyBytes, len(c.IDCardEncKey)))
	}

	switch {
	case c.IDCardHMACKey == "":
		problems = append(problems, fmt.Sprintf("%s 未配置（身份证号去重哈希密钥）", EnvIDCardHMACKey))
	case len(c.IDCardHMACKey) < MinHMACKeyBytes:
		problems = append(problems,
			fmt.Sprintf("%s 过短：需 ≥ %d 字节，实际 %d 字节", EnvIDCardHMACKey, MinHMACKeyBytes, len(c.IDCardHMACKey)))
	}

	switch {
	case c.SessionSecret == "":
		problems = append(problems, fmt.Sprintf("%s 未配置（会话签名密钥）", EnvSessionSecret))
	case len(c.SessionSecret) < MinHMACKeyBytes:
		problems = append(problems,
			fmt.Sprintf("%s 过短：需 ≥ %d 字节，实际 %d 字节", EnvSessionSecret, MinHMACKeyBytes, len(c.SessionSecret)))
	}

	if len(problems) > 0 {
		return fmt.Errorf("强制密钥校验失败，服务拒绝启动：\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// Addr 返回监听地址。
func (c *Config) Addr() string { return c.Host + ":" + c.Port }

// SMSEnabled 短信通道是否可用（fail-closed：三项齐备才启用）。
func (c *Config) SMSEnabled() bool {
	return c.SMSProvider != "" && c.SMSSecretID != "" && c.SMSSecretKey != ""
}

// LLMEnabled AI 网关是否可用（fail-closed）。
func (c *Config) LLMEnabled() bool { return c.LLMProvider != "" && c.LLMAPIKey != "" }

// WXPayEnabled 微信支付是否可用（fail-closed）。
func (c *Config) WXPayEnabled() bool { return c.WXPayMchID != "" && c.WXPayAPIv3Key != "" }

// AlipayEnabled 支付宝是否可用（fail-closed）。
func (c *Config) AlipayEnabled() bool { return c.AlipayAppID != "" && c.AlipayPrivKey != "" }

// ErrNoSecret 供调用方 errors.Is 判定的哨兵错误。
var ErrNoSecret = errors.New("强制密钥未配置")

func boolDefault(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
