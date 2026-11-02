package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var ErrNotConfigured = errors.New("Redis 未配置")

// Client 是限频等场景所需的最小 Redis 能力。
type Client interface {
	Ping(context.Context) error
	Close() error
	EvalInt(ctx context.Context, script string, keys []string, args ...any) (int64, error)
}

// GoRedis 基于 go-redis 的实现，连接失败时由调用方降级。
type GoRedis struct {
	client *goredis.Client
}

// New 建立连接并 Ping 校验；调用方应在失败时降级到进程内限频。
func New(addr, password string, database int) (*GoRedis, error) {
	if addr == "" {
		return nil, ErrNotConfigured
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     password,
		DB:           database,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &GoRedis{client: client}, nil
}

func (g *GoRedis) Ping(ctx context.Context) error { return g.client.Ping(ctx).Err() }

func (g *GoRedis) Close() error { return g.client.Close() }

func (g *GoRedis) EvalInt(ctx context.Context, script string, keys []string, args ...any) (int64, error) {
	return g.client.Eval(ctx, script, keys, args...).Int64()
}

type Unconfigured struct{}

func (Unconfigured) Ping(context.Context) error { return ErrNotConfigured }
func (Unconfigured) Close() error               { return nil }
func (Unconfigured) EvalInt(context.Context, string, []string, ...any) (int64, error) {
	return 0, ErrNotConfigured
}
