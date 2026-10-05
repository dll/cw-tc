package redis

import (
	"context"
	"errors"
)

var ErrNotConfigured = errors.New("Redis 未配置")

type Client interface {
	Ping(context.Context) error
	Close() error
}

type Unconfigured struct{}

func (Unconfigured) Ping(context.Context) error { return ErrNotConfigured }
func (Unconfigured) Close() error               { return nil }
