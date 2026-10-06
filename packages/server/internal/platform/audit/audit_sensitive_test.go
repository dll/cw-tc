package audit

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryWriterDoesNotStoreSensitiveRawValues(t *testing.T) {
	m := NewMemory()
	entry := Entry{
		Action: "student.reveal",
		Before: []byte(`{"id_card":"sample-id-card"}`),
		After:  []byte(`{"phone":"sample-phone"}`),
	}
	if err := m.Write(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	for _, stored := range m.Entries() {
		if strings.Contains(string(stored.Before), "sample-id-card") || strings.Contains(string(stored.After), "sample-phone") {
			t.Fatal("审计记录不得存储敏感原文")
		}
	}
}
