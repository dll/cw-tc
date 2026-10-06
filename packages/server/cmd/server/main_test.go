package main

import "testing"

func TestLogLevel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{"debug", "debug", "DEBUG"},
		{"warn", "warn", "WARN"},
		{"error", "error", "ERROR"},
		{"default", "other", "INFO"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.value)
			if got := logLevel().String(); got != tc.want {
				t.Fatalf("日志级别 = %s，期望 %s", got, tc.want)
			}
		})
	}
}
