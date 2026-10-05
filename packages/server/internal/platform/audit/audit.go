package audit

import (
	"context"
	"errors"
	"sync"
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
	m.entries = append(m.entries, e)
	return nil
}

func (m *Memory) Entries() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Entry(nil), m.entries...)
}

var ErrNotConfigured = errors.New("审计存储未配置")
