package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
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

func TestGoRedisEvalAndPing(t *testing.T) {
	mr := miniredis.RunT(t)
	client, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("连接 miniredis 失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
	count, err := client.EvalInt(ctx, "return redis.call('INCR', KEYS[1])", []string{"k"})
	if err != nil || count != 1 {
		t.Fatalf("EvalInt 期望 1，实际 %d / %v", count, err)
	}
}

func TestNewRejectsEmptyAndUnreachable(t *testing.T) {
	if _, err := New("", "", 0); err != ErrNotConfigured {
		t.Fatalf("空地址应返回 ErrNotConfigured，实际 %v", err)
	}
	if _, err := New("127.0.0.1:1", "", 0); err == nil {
		t.Fatal("不可达地址应返回错误")
	}
}

func TestUnconfiguredClient(t *testing.T) {
	var c Client = Unconfigured{}
	if err := c.Ping(context.Background()); err != ErrNotConfigured {
		t.Fatalf("Ping 错误不符: %v", err)
	}
	if _, err := c.EvalInt(context.Background(), "return 1", []string{"k"}); err != ErrNotConfigured {
		t.Fatalf("EvalInt 错误不符: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close 不应报错: %v", err)
	}
}
