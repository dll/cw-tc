package audit

import (
	"context"
	"testing"
)

func TestNopWriterAcceptsEntry(t *testing.T) {
	if err := (Nop{}).Write(context.Background(), Entry{Action: "health.check"}); err != nil {
		t.Fatalf("Nop 审计写入应成功: %v", err)
	}
}
