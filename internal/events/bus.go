package events

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

// timeNow is a package-level var for test injection.
var timeNow = time.Now

// fanOutHook is called by fanOut before dispatching each event.
// It is nil in production and may be set in tests to inject delays.
// Access is guarded by fanOutHookMu.
var (
	fanOutHookMu sync.RWMutex
	fanOutHook   func()
)

// Kind is the discriminator for all bus events.
type Kind string

const (
	KindProcessStarted    Kind = "process.started"
	KindProcessStopped    Kind = "process.stopped"
	KindProcessCrashed    Kind = "process.crashed"
	KindProcessRestarting Kind = "process.restarting"
	KindLogLine           Kind = "log.line"
	KindAttentionNeeded   Kind = "attention.needed"
	KindAttentionCleared  Kind = "attention.cleared"
	KindConfigChanged     Kind = "config.changed"
	KindPermissionReq     Kind = "permission.request"
	KindPermissionDec     Kind = "permission.decision"
	KindResourceSample    Kind = "resource.sample"
)

// Event is the envelope for all bus messages.
type Event struct {
	Kind      Kind
	Timestamp time.Time
	payload   any
}

// NewEvent constructs an Event with the given kind and payload.
func NewEvent(kind Kind, payload any) Event {
	return Event{Kind: kind, Timestamp: timeNow(), payload: payload}
}

// Payload returns the typed payload. Callers must type-assert to the correct type.
func (e Event) Payload() any { return e.payload }

// ProcessEvent carries process lifecycle changes.
type ProcessEvent struct {
	State domain.ProcessState
}

// LogEvent carries a single log line.
type LogEvent struct {
	Line domain.LogLine
}

// AttentionEvent carries an attention signal.
type AttentionEvent struct {
	Event domain.AttentionEvent
}

// ConfigChangedEvent carries the diff when smooth.yml changes.
type ConfigChangedEvent struct {
	Added    []string          // process names added
	Removed  []string          // process names removed
	Modified []ConfigFieldDiff // fields changed on existing processes
}

type ConfigFieldDiff struct {
	Process  string
	Field    string
	OldValue string
	NewValue string
}

// Publisher is the interface for publishing events to the bus.
type Publisher interface {
	// Publish puts an event on the bus. Never blocks — if the bus buffer is
	// full, the oldest event is dropped and a metric is incremented.
	Publish(e Event)
}

// Subscriber is the interface for subscribing to the bus.
type Subscriber interface {
	// Subscribe returns a channel that receives all events published to the bus.
	// The channel is buffered (default 256). If the subscriber channel fills,
	// that subscriber is skipped for that event (slow-subscriber protection).
	// The returned func unsubscribes and stops delivery.
	Subscribe() (<-chan Event, func())
}

// Bus is the combined Publisher + Subscriber + lifecycle interface.
type Bus interface {
	Publisher
	Subscriber
	DroppedCount() int64
	Close()
}

const (
	busBufferSize        = 4096
	subscriberBufferSize = 256
)

type subscriber struct {
	ch        chan Event
	closeOnce sync.Once
}

func (s *subscriber) close() {
	s.closeOnce.Do(func() { close(s.ch) })
}

// bus is the concrete Bus implementation.
type bus struct {
	in      chan Event
	mu      sync.RWMutex
	subs    []*subscriber
	dropped atomic.Int64
	done    chan struct{}
	once    sync.Once
}

// NewBus creates and starts a new Bus.
func NewBus() Bus {
	b := &bus{
		in:   make(chan Event, busBufferSize),
		done: make(chan struct{}),
	}
	go b.fanOut()
	return b
}

func (b *bus) Publish(e Event) {
	select {
	case b.in <- e:
	default:
		// bus buffer full — drop oldest by draining one, then re-sending
		select {
		case <-b.in:
			b.dropped.Add(1)
		default:
		}
		select {
		case b.in <- e:
		default:
		}
	}
}

func (b *bus) Subscribe() (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, subscriberBufferSize)}
	b.mu.Lock()
	b.subs = append(b.subs, s)
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		for i, sub := range b.subs {
			if sub == s {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
		s.close() // close after removing from subs so fanOut won't send to it
	}
	return s.ch, unsub
}

func (b *bus) DroppedCount() int64 {
	return b.dropped.Load()
}

func (b *bus) Close() {
	b.once.Do(func() {
		close(b.done)
	})
}

func (b *bus) fanOut() {
	for {
		select {
		case <-b.done:
			b.mu.RLock()
			for _, s := range b.subs {
				s.close()
			}
			b.mu.RUnlock()
			return
		case e := <-b.in:
			fanOutHookMu.RLock()
			hook := fanOutHook
			fanOutHookMu.RUnlock()
			if hook != nil {
				hook()
			}
			b.mu.RLock()
			for _, s := range b.subs {
				select {
				case s.ch <- e:
				default:
					b.dropped.Add(1) // slow subscriber — increment drop counter
				}
			}
			b.mu.RUnlock()
		}
	}
}
