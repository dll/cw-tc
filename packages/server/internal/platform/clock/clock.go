package clock

import (
	"context"
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func Real() Clock { return realClock{} }

type contextKey struct{}

func With(ctx context.Context, c Clock) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, c)
}

func From(ctx context.Context) Clock {
	if c, ok := ctx.Value(contextKey{}).(Clock); ok && c != nil {
		return c
	}
	return Real()
}

type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T }
