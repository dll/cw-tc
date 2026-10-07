package config

import (
	"strings"
	"testing"
)

func TestPaymentEnabledRequiresFullCredentialSet(t *testing.T) {
	base := func(get func(string) string) *Config {
		c, err := LoadFrom(func(k string) string {
			switch k {
			case EnvIDCardEncKey:
				return strings.Repeat("a", IDCardEncKeyBytes)
			case EnvIDCardHMACKey, EnvSessionSecret:
				return strings.Repeat("b", MinHMACKeyBytes)
			default:
				return get(k)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// 全空 → 关闭
	if c := base(func(string) string { return "" }); c.WXPayEnabled() || c.AlipayEnabled() {
		t.Fatal("全空应关闭")
	}
	// 微信全套 → 开启
	wx := map[string]string{
		EnvWXPayMchID: "123", EnvWXPayAppID: "wx123", EnvWXPayAPIv3Key: "k",
		EnvWXPaySerialNo: "s", EnvWXPayKeyPath: "/p", EnvWXPayNotify: "https://x",
	}
	if c := base(func(k string) string { return wx[k] }); !c.WXPayEnabled() {
		t.Fatal("微信全套应开启")
	}
	// 微信缺一项 → 关闭
	if c := base(func(k string) string {
		if k == EnvWXPaySerialNo {
			return ""
		}
		return wx[k]
	}); c.WXPayEnabled() {
		t.Fatal("微信缺证书序列号应关闭")
	}
	// 支付宝全套 → 开启
	ali := map[string]string{
		EnvAlipayAppID: "1", EnvAlipayPrivKey: "/p", EnvAlipayPubKey: "/p",
		EnvAlipayNotify: "https://x", EnvAlipayGateway: "https://openapi.alipay.com/gateway.do",
	}
	if c := base(func(k string) string { return ali[k] }); !c.AlipayEnabled() {
		t.Fatal("支付宝全套应开启")
	}
}
