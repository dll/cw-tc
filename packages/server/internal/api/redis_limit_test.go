package api

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/cw-tc/server/internal/config"
)

func TestNewSharesRateLimitAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := &config.Config{RedisAddr: mr.Addr()}
	ctx := context.Background()

	first := New(cfg)
	second := New(cfg)
	for i := 0; i < 2; i++ {
		if ok, err := first.rateLimiter.Allow(ctx, "login|1.2.3.4", 2, time.Minute); err != nil || !ok {
			t.Fatalf("实例一第 %d 次应放行: %v", i+1, err)
		}
	}
	if ok, err := second.rateLimiter.Allow(ctx, "login|1.2.3.4", 2, time.Minute); err != nil || ok {
		t.Fatalf("实例二应共享计数并拒绝: ok=%v err=%v", ok, err)
	}
}
