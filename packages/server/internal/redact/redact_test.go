package redact

import (
	"strings"
	"testing"
)

func TestIDCard(t *testing.T) {
	cases := []struct{ in, want string }{
		// 上游 保密说明.md 口径：前 6 + 8 星 + 后 4
		{"110101199001011234", "110101********1234"},
		{"11010119900101123X", "110101********123X"},
		{"11010119900101123x", "110101********123x"},
		{"110101", Mask}, // 过短 -> 整体掩码
		{"", Mask},
	}
	for _, tc := range cases {
		if got := IDCard(tc.in); got != tc.want {
			t.Errorf("IDCard(%q) = %q，期望 %q", tc.in, got, tc.want)
		}
	}
}

func TestPhone(t *testing.T) {
	cases := []struct{ in, want string }{
		{"13812345678", "138****5678"},
		{"19900001111", "199****1111"},
		{"1381234", Mask}, // 过短
	}
	for _, tc := range cases {
		if got := Phone(tc.in); got != tc.want {
			t.Errorf("Phone(%q) = %q，期望 %q", tc.in, got, tc.want)
		}
	}
}

func TestString_ReplacesBoth(t *testing.T) {
	in := "login failed for 110101199001011234 from 13812345678"
	got := String(in)
	if strings.Contains(got, "110101199001011234") {
		t.Errorf("身份证号未被过滤: %q", got)
	}
	if strings.Contains(got, "13812345678") {
		t.Errorf("手机号未被过滤: %q", got)
	}
	want := "login failed for 110101********1234 from 138****5678"
	if got != want {
		t.Errorf("String() = %q，期望 %q", got, want)
	}
}

// TestString_LeavesNormalText 保证过滤器不会误伤课程号、题号、版本号等正常文本。
func TestString_LeavesNormalText(t *testing.T) {
	texts := []string{
		"GET /api/v1/courses/07-CN/lectures/10",
		"question L2-07-01 scored 12.5",
		"CW-CN-M202600137",
		"build 1.0.42 commit 9f6c5c6",
		"",
	}
	for _, s := range texts {
		if got := String(s); got != s {
			t.Errorf("正常文本被误改: %q -> %q", s, got)
		}
	}
}

// TestString_DoesNotTouch19DigitRuns 边界：19 位及以上的长数字串不应被当作身份证号截取。
func TestString_DoesNotTouch19DigitRuns(t *testing.T) {
	in := "order 1101011990010112345 trace"
	if got := String(in); got != in {
		t.Errorf("19 位数字串不应被替换，实际: %q", got)
	}
}

// TestString_Idempotent 脱敏结果再脱敏应保持不变（掩码里不含可被再次匹配的模式）。
func TestString_Idempotent(t *testing.T) {
	once := String("id 110101199001011234 phone 13812345678")
	twice := String(once)
	if once != twice {
		t.Errorf("重复脱敏结果不一致:\n once = %q\ntwice = %q", once, twice)
	}
}

// TestGate_NoSensitiveAfterString 守门测试：
// 出站文本经过 String 之后，两个检测器都必须报 false。
// 这是"日志必须过滤"这条铁律的自动化证据。
func TestGate_NoSensitiveAfterString(t *testing.T) {
	raw := "POST /x?id=110101199001011234&phone=13812345678 失败"
	clean := String(raw)
	if ContainsIDCard(clean) {
		t.Errorf("过滤后仍能检出身份证号: %q", clean)
	}
	if ContainsPhone(clean) {
		t.Errorf("过滤后仍能检出手机号: %q", clean)
	}
}
