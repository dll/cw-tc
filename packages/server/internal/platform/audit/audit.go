package audit

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/cw-tc/server/internal/redact"
)

type Entry struct {
	ActorType  string
	ActorID    int64
	Action     string
	TargetType string
	TargetID   int64
	Before     []byte
	After      []byte
}

type Writer interface {
	Write(context.Context, Entry) error
}

type Nop struct{}

func (Nop) Write(context.Context, Entry) error { return nil }

type Memory struct {
	mu      sync.Mutex
	entries []Entry
	err     error
}

func NewMemory() *Memory { return &Memory{} }

func (m *Memory) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *Memory) Write(_ context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	e.Before = redactAuditPayload(e.Before)
	e.After = redactAuditPayload(e.After)
	m.entries = append(m.entries, e)
	return nil
}

func redactAuditPayload(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(payload, &value) == nil {
		redacted, _ := json.Marshal(redactAuditValue(value))
		return redacted
	}
	return []byte(redact.String(string(payload)))
}

func redactAuditValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		for key, nested := range item {
			if key == "id_card" || key == "idCard" || key == "phone" || key == "mobile" {
				item[key] = "[REDACTED]"
				continue
			}
			item[key] = redactAuditValue(nested)
		}
	case []any:
		for index, nested := range item {
			item[index] = redactAuditValue(nested)
		}
	}
	return value
}

func (m *Memory) Entries() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Entry(nil), m.entries...)
}

var ErrNotConfigured = errors.New("审计存储未配置")
