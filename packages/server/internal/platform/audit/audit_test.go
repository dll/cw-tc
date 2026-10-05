package audit

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryWriterStoresEntriesAndSurfacesErrors(t *testing.T) {
	m := NewMemory()
	entry := Entry{Action: "course.price_change", TargetType: "course", TargetID: 7}
	if err := m.Write(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	got := m.Entries()
	if len(got) != 1 || got[0].Action != entry.Action || got[0].TargetID != entry.TargetID {
		t.Fatalf("审计记录错误: %#v", got)
	}
	wantErr := errors.New("store down")
	m.SetError(wantErr)
	if !errors.Is(m.Write(context.Background(), entry), wantErr) {
		t.Fatal("审计存储错误未向上传递")
	}
}
