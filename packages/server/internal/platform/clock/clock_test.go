package clock

import (
	"context"
	"testing"
	"time"
)

func TestFromUsesInjectedClock(t *testing.T) {
	want := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	got := From(With(context.Background(), Fixed{T: want})).Now()
	if !got.Equal(want) {
		t.Fatalf("时钟 = %v，期望 %v", got, want)
	}
}

func TestFromFallsBackToUTCClock(t *testing.T) {
	if From(context.Background()).Now().Location() != time.UTC {
		t.Fatal("默认时钟必须使用 UTC")
	}
}
