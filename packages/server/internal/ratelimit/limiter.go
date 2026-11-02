// Package ratelimit 提供固定窗口限频：生产多实例共享 Redis，Redis 故障时降级到进程内。
package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cw-tc/server/internal/platform/redis"
)

// Limiter 是限频器。Allow 返回 false 表示已超限。
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

type memoryEntry struct {
	count int
	reset time.Time
}

// Memory 是单实例内存实现，仅在未配置 Redis 或 Redis 故障时兜底。
type Memory struct {
	mu      sync.Mutex
	entries map[string]*memoryEntry
}

func NewMemory() *Memory { return &Memory{entries: map[string]*memoryEntry{}} }

func (m *Memory) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[key]
	if !ok || !now.Before(entry.reset) {
		entry = &memoryEntry{reset: now.Add(window)}
		m.entries[key] = entry
	}
	entry.count++
	if len(m.entries) > 20000 {
		for k, v := range m.entries {
			if !now.Before(v.reset) {
				delete(m.entries, k)
			}
		}
	}
	return entry.count <= limit, nil
}

const redisScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`

const keyPrefix = "cwtc:ratelimit:"

// Redis 使用 INCR + PEXPIRE 原子计数，所有实例共享同一窗口。
type Redis struct {
	client redis.Client
}

func NewRedis(client redis.Client) *Redis { return &Redis{client: client} }

func (r *Redis) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	count, err := r.client.EvalInt(ctx, redisScript, []string{keyPrefix + key}, window.Milliseconds())
	if err != nil {
		return false, err
	}
	return count <= int64(limit), nil
}

// Failover 优先使用 Redis；Redis 不可用时降级到内存，避免限频失效。
type Failover struct {
	primary  Limiter
	fallback Limiter

	mu       sync.Mutex
	retryAt  time.Time
	cooldown time.Duration
}

func NewFailover(primary, fallback Limiter) *Failover {
	return &Failover{primary: primary, fallback: fallback, cooldown: 30 * time.Second}
}

func (f *Failover) inCooldown() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return time.Now().Before(f.retryAt)
}

func (f *Failover) markFailed() {
	f.mu.Lock()
	f.retryAt = time.Now().Add(f.cooldown)
	f.mu.Unlock()
}

func (f *Failover) markHealthy() {
	f.mu.Lock()
	f.retryAt = time.Time{}
	f.mu.Unlock()
}

func (f *Failover) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if f.primary != nil && !f.inCooldown() {
		allowed, err := f.primary.Allow(ctx, key, limit, window)
		if err == nil {
			f.markHealthy()
			return allowed, nil
		}
		f.markFailed()
	}
	if f.fallback == nil {
		return false, fmt.Errorf("限频器未配置")
	}
	return f.fallback.Allow(ctx, key, limit, window)
}
