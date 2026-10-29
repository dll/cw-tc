package api

import "testing"

func TestDeviceLabel(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0": "Windows · Chrome",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Safari/605":    "iOS · Safari",
		"Mozilla/5.0 (Linux; Android 14) Chrome/120.0":           "Android · Chrome",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X) Edg/120.0":      "macOS · Edge",
		"Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0":          "Linux · Firefox",
		"curl/8.0": "未知设备",
	}
	for userAgent, want := range cases {
		if got := deviceLabel(userAgent); got != want {
			t.Errorf("deviceLabel(%q)=%q，期望 %q", userAgent, got, want)
		}
	}
}
