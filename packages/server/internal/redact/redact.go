// Package redact 提供敏感信息脱敏与日志过滤。
//
// 对应铁律（PRD v2 §9.2 / AGENTS.md 第 3 条）：
// 身份证号与手机号**绝不出现在日志、埋点与错误上报中**。
// 该包是这条铁律在代码层的落点 —— 所有出站文本（访问日志、错误信息、埋点）
// 必须先过 String 再输出。
//
// 注意：本包只做"输出侧过滤"。真正的合规前提是
// **身份证号明文不落库**（AES-256-GCM 密文）与**登录走 POST body 不进 query**。
package redact

import "regexp"

const (
	// Mask 为脱敏使用的固定掩码（8 个星号）。
	Mask = "********"
)

var (
	// 18 位身份证号：17 位数字 + 校验位（数字或 X/x）。
	// 用 \b 限定词边界，避免命中 19 位及以上的数字串内部。
	idCardRe = regexp.MustCompile(`\b\d{17}[\dXx]\b`)

	// 中国大陆手机号：1 开头共 11 位。
	phoneRe = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
)

// IDCard 对单个身份证号脱敏：前 6 + 8 星 + 后 4（上游 保密说明.md 的口径）。
// 长度不足 11 位时原样返回掩码，避免把短串"脱敏"成更长的可疑串。
func IDCard(s string) string {
	if len(s) < 11 {
		return Mask
	}
	return s[:6] + Mask + s[len(s)-4:]
}

// Phone 对单个手机号脱敏：前 3 + 4 星 + 后 4。
func Phone(s string) string {
	if len(s) < 8 {
		return Mask
	}
	return s[:3] + "****" + s[len(s)-4:]
}

// String 对任意文本做整串过滤：先替换身份证号，再替换手机号。
//
// 顺序不可颠倒 —— 虽然 18 位串内部不存在词边界、手机号规则不会误伤，
// 但先处理更长、更敏感的身份证号更稳妥。
func String(s string) string {
	if s == "" {
		return s
	}
	s = idCardRe.ReplaceAllStringFunc(s, IDCard)
	s = phoneRe.ReplaceAllStringFunc(s, Phone)
	return s
}

// ContainsIDCard 报告文本中是否仍存在身份证号（用于自检与单测守门）。
func ContainsIDCard(s string) bool { return idCardRe.MatchString(s) }

// ContainsPhone 报告文本中是否仍存在手机号（用于自检与单测守门）。
func ContainsPhone(s string) bool { return phoneRe.MatchString(s) }
