package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryLimitAndReset(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if ok, _ := m.Allow(ctx, "k", 2, 30*time.Millisecond); !ok {
			t.Fatalf("第 %d 次应放行", i+1)
		}
	}
	if ok, _ := m.Allow(ctx, "k", 2, 30*time.Millisecond); ok {
		t.Fatal("超过上限应拒绝")
	}
	time.Sleep(40 * time.Millisecond)
	if ok, _ := m.Allow(ctx, "k", 2, 30*time.Millisecond); !ok {
		t.Fatal("窗口重置后应放行")
	}
}

type fakeRedis struct {
	counts map[string]int64
	err    error
	calls  int
}

func (f *fakeRedis) Ping(context.Context) error { return f.err }
func (f *fakeRedis) Close() error               { return nil }
func (f *fakeRedis) EvalInt(_ context.Context, _ string, keys []string, _ ...any) (int64, error) {
	f.calls++
	if f.err != nil {
		return 0, f.err
	}
	if f.counts == nil {
		f.counts = map[string]int64{}
	}
	f.counts[keys[0]]++
	return f.counts[keys[0]], nil
}

func TestRedisSharesCounterByKey(t *testing.T) {
	r := NewRedis(&fakeRedis{})
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if ok, err := r.Allow(ctx, "login|1.1.1.1", 2, time.Minute); err != nil || !ok {
			t.Fatalf("第 %d 次应放行: %v", i+1, err)
		}
	}
	if ok, _ := r.Allow(ctx, "login|1.1.1.1", 2, time.Minute); ok {
		t.Fatal("同一 key 超过上限应拒绝")
	}
	if ok, _ := r.Allow(ctx, "login|2.2.2.2", 2, time.Minute); !ok {
		t.Fatal("不同 key 不应互相影响")
	}
}

func TestFailoverFallsBackOnRedisError(t *testing.T) {
	client := &fakeRedis{err: errors.New("down")}
	f := NewFailover(NewRedis(client), NewMemory())
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if ok, err := f.Allow(ctx, "k", 2, time.Minute); err != nil || !ok {
			t.Fatalf("Redis 故障时应由内存兜底: %v", err)
		}
	}
	if ok, _ := f.Allow(ctx, "k", 2, time.Minute); ok {
		t.Fatal("兜底限频仍应生效")
	}
	if client.calls != 1 {
		t.Fatalf("熔断期内不应反复重连 Redis，实际调用 %d 次", client.calls)
	}
}

func TestFailoverUsesPrimaryAndNoFallback(t *testing.T) {
	f := NewFailover(NewRedis(&fakeRedis{counts: map[string]int64{keyPrefix + "k": 9}}), NewMemory())
	if ok, err := f.Allow(context.Background(), "k", 10, time.Minute); !ok || err != nil {
		t.Fatal("未超限应放行")
	}
	if ok, err := f.Allow(context.Background(), "k", 10, time.Minute); ok || err != nil {
		t.Fatal("Redis 返回超限应拒绝")
	}
	empty := &Failover{}
	if _, err := empty.Allow(context.Background(), "k", 1, time.Minute); err == nil {
		t.Fatal("缺少兜底应报错")
	}
}
