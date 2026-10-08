// Package config 从环境变量加载服务配置。
//
// 安全默认（PRD v2 §12.2，沿用 ccit 约定）：
//   - 强制密钥（身份证加密密钥 / HMAC 密钥 / 会话密钥）缺失或过短 —— 服务拒绝启动，
//     绝不退回"明文暂存"这类降级路径；
//   - 其余凭据（短信 / 支付 / 大模型）未配置 —— 对应功能 fail-closed 不启用。
//
// 环境变量名与 deploy/.env.example 保持一一对应。
//
// 数据库边界（2026-10-06 明确）：
//   - 开发与测试：SQLite（`CWTC_DB_DRIVER=sqlite`，不依赖本机 PostgreSQL / Redis）；
//   - 生产：腾讯云 PostgreSQL（`CWTC_DB_DRIVER=postgres`，沿用 CCIT 的部署约定）；
//   - **`DEPLOY_ENV=production` 时拒绝 SQLite** —— 拿开发库跑生产是最危险的一类配置事故。
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
	EnvWXPayAppID    = "CWTC_WXPAY_APP_ID"
	EnvWXPaySerialNo = "CWTC_WXPAY_SERIAL_NO"
	EnvWXPayKeyPath  = "CWTC_WXPAY_PRIVATE_KEY_PATH"
	EnvWXPayNotify   = "CWTC_WXPAY_NOTIFY_URL"
	EnvAlipayAppID   = "CWTC_ALIPAY_APP_ID"
	EnvAlipayPrivKey = "CWTC_ALIPAY_PRIVATE_KEY_PATH"
	EnvAlipayPubKey  = "CWTC_ALIPAY_PUBLIC_KEY_PATH"
	EnvAlipayNotify  = "CWTC_ALIPAY_NOTIFY_URL"
	EnvAlipayGateway = "CWTC_ALIPAY_GATEWAY"

	// 部署环境与数据库（开发/测试 SQLite；生产 PostgreSQL）
	EnvDeployEnv = "DEPLOY_ENV"
	EnvDBDriver  = "CWTC_DB_DRIVER"
	EnvDBDSN     = "CWTC_DB_DSN"

	// 资源文件根目录（签名下载服务从此目录按 storage_key 取文件）
	EnvContentDir = "CWTC_CONTENT_DIR"

	// 对象存储 COS（视频/音频资源；未配置 = 该功能 fail-closed，走文件服务）
	EnvCosSecretID  = "CWTC_COS_SECRET_ID"
	EnvCosSecretKey = "CWTC_COS_SECRET_KEY"
	EnvCosBucket    = "CWTC_COS_BUCKET"
	EnvCosRegion    = "CWTC_COS_REGION"
	EnvCosBaseURL   = "CWTC_COS_BASE_URL"

	// 初始账号种子（`seed-admin` 命令读取；密码不写死、缺失则跳过该账号）
	EnvAdminAccount   = "CWTC_ADMIN_ACCOUNT"
	EnvAdminPassword  = "CWTC_ADMIN_PASSWORD"
	EnvAdminName      = "CWTC_ADMIN_NAME"
	EnvOpsAccount     = "CWTC_OPS_ACCOUNT"
	EnvOpsPassword    = "CWTC_OPS_PASSWORD"
	EnvOpsName        = "CWTC_OPS_NAME"
	EnvTeacherAccount = "CWTC_TEACHER_ACCOUNT"
	EnvTeacherPasswd  = "CWTC_TEACHER_PASSWORD"
	EnvTeacherName    = "CWTC_TEACHER_NAME"

	// 默认学员种子（手机号即登录账号，铁律 3；缺密码则跳过）
	EnvStudentAccount  = "CWTC_STUDENT_ACCOUNT"
	EnvStudentPassword = "CWTC_STUDENT_PASSWORD"
	EnvStudentName     = "CWTC_STUDENT_NAME"
	EnvStudentIDCard   = "CWTC_STUDENT_ID_CARD"

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
	WXPayAppID    string
	WXPaySerialNo string
	WXPayKeyPath  string
	WXPayNotify   string
	AlipayAppID   string
	AlipayPrivKey string
	AlipayPubKey  string
	AlipayNotify  string
	AlipayGateway string

	PublicStatusEnabled bool
	LogRedactEnabled    bool

	// 部署环境与数据库（开发/测试 SQLite；生产 PostgreSQL）
	DeployEnv string
	DBDriver  string
	DBDSN     string

	// 资源文件根目录（签名下载服务从此目录取文件）
	ContentDir string

	// 对象存储 COS
	CosSecretID  string
	CosSecretKey string
	CosBucket    string
	CosRegion    string
	CosBaseURL   string
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
		WXPayAppID:    strings.TrimSpace(get(EnvWXPayAppID)),
		WXPaySerialNo: strings.TrimSpace(get(EnvWXPaySerialNo)),
		WXPayKeyPath:  strings.TrimSpace(get(EnvWXPayKeyPath)),
		WXPayNotify:   strings.TrimSpace(get(EnvWXPayNotify)),
		AlipayAppID:   strings.TrimSpace(get(EnvAlipayAppID)),
		AlipayPrivKey: strings.TrimSpace(get(EnvAlipayPrivKey)),
		AlipayPubKey:  strings.TrimSpace(get(EnvAlipayPubKey)),
		AlipayNotify:  strings.TrimSpace(get(EnvAlipayNotify)),
		AlipayGateway: strings.TrimSpace(get(EnvAlipayGateway)),

		// 部署环境与数据库（默认按开发环境；生产必须显式设 postgres）
		DeployEnv: strings.ToLower(strings.TrimSpace(get(EnvDeployEnv))),
		DBDriver:  strings.ToLower(strings.TrimSpace(get(EnvDBDriver))),
		DBDSN:     strings.TrimSpace(get(EnvDBDSN)),

		ContentDir: strings.TrimSpace(get(EnvContentDir)),

		CosSecretID:  strings.TrimSpace(get(EnvCosSecretID)),
		CosSecretKey: strings.TrimSpace(get(EnvCosSecretKey)),
		CosBucket:    strings.TrimSpace(get(EnvCosBucket)),
		CosRegion:    strings.TrimSpace(get(EnvCosRegion)),
		CosBaseURL:   strings.TrimSpace(get(EnvCosBaseURL)),

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
	if c.DeployEnv == "" {
		c.DeployEnv = "development"
	}
	if c.DBDriver == "" {
		c.DBDriver = "sqlite" // 开发/测试默认；生产必须显式设为 postgres
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

	switch c.DBDriver {
	case "sqlite", "postgres":
	default:
		problems = append(problems,
			fmt.Sprintf("%s 取值非法：%q（允许 sqlite / postgres）", EnvDBDriver, c.DBDriver))
	}
	if c.IsProduction() && c.DBDriver == "sqlite" {
		problems = append(problems, fmt.Sprintf(
			"生产环境不得使用 SQLite（%s=production 且 %s=sqlite）："+
				"拿开发库跑生产会造成「测试通过、线上丢数据」，请显式设为 postgres",
			EnvDeployEnv, EnvDBDriver))
	}

	if len(problems) > 0 {
		return fmt.Errorf("配置校验失败，服务拒绝启动：\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// IsProduction 是否生产环境。
func (c *Config) IsProduction() bool { return c.DeployEnv == "production" }

// Addr 返回监听地址。
func (c *Config) Addr() string { return c.Host + ":" + c.Port }

// SMSEnabled 短信通道是否可用（fail-closed：三项齐备才启用）。
func (c *Config) SMSEnabled() bool {
	return c.SMSProvider != "" && c.SMSSecretID != "" && c.SMSSecretKey != ""
}

// LLMEnabled AI 网关是否可用（fail-closed）。
func (c *Config) LLMEnabled() bool { return c.LLMProvider != "" && c.LLMAPIKey != "" }

// WXPayEnabled 微信支付是否可用（fail-closed：全套齐备才启用）。
func (c *Config) WXPayEnabled() bool {
	return c.WXPayMchID != "" && c.WXPayAppID != "" && c.WXPayAPIv3Key != "" &&
		c.WXPaySerialNo != "" && c.WXPayKeyPath != "" && c.WXPayNotify != ""
}

// AlipayEnabled 支付宝是否可用（fail-closed：全套齐备才启用）。
func (c *Config) AlipayEnabled() bool {
	return c.AlipayAppID != "" && c.AlipayPrivKey != "" && c.AlipayPubKey != "" &&
		c.AlipayNotify != "" && c.AlipayGateway != ""
}

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
