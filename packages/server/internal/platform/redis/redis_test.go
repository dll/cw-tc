package redis

import (
	"context"
	"errors"
	"testing"
)

func TestUnconfiguredFailsClosed(t *testing.T) {
	var client Unconfigured
	if !errors.Is(client.Ping(context.Background()), ErrNotConfigured) {
		t.Fatal("未配置 Redis 时必须 fail-closed")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("未配置 Redis 的 Close 应可安全调用: %v", err)
	}
}
