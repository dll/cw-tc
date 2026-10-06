package clock

import (
	"context"
	"testing"
	"time"
)

func TestWithNilKeepsContextAndFixedClockIsInjectable(t *testing.T) {
	base := context.Background()
	if got := With(base, nil); got != base {
		t.Fatal("注入 nil 时应保留原 context")
	}
	want := time.Date(2026, time.October, 6, 0, 0, 0, 0, time.UTC)
	ctx := With(base, Fixed{T: want})
	if got := From(ctx).Now(); !got.Equal(want) {
		t.Fatalf("注入时钟 = %v，期望 %v", got, want)
	}
}
