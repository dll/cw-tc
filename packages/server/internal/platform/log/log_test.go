package log

import "testing"

func TestDefaultReturnsLogger(t *testing.T) {
	if Default() == nil {
		t.Fatal("Default 必须返回日志器")
	}
}
