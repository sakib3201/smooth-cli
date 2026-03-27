package notify

import (
	"fmt"
	"sync"

	"github.com/gen2brain/beeep"
)

type Notifier interface {
	Notify(title, body string) error
}

func New() Notifier {
	return &BeeepNotifier{}
}

func NewMock() *MockNotifier {
	return &MockNotifier{}
}

type BeeepNotifier struct{}

func (b *BeeepNotifier) Notify(title, body string) error {
	if err := beeep.Notify(title, body, ""); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return nil
}

type MockNotifier struct {
	mu    sync.Mutex
	calls []NotifyCall
}

type NotifyCall struct {
	Title string
	Body  string
}

func (m *MockNotifier) Notify(title, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, NotifyCall{Title: title, Body: body})
	return nil
}

func (m *MockNotifier) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *MockNotifier) Calls() []NotifyCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]NotifyCall, len(m.calls))
	copy(out, m.calls)
	return out
}
