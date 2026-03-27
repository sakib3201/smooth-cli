package events_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/smoothcli/smooth-cli/internal/events"
)

func newTestEvent(kind events.Kind) events.Event {
	return events.NewEvent(kind, nil)
}

func TestBus_DeliversEvent_ToSingleSubscriber(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	ch, _ := b.Subscribe()
	e := newTestEvent(events.KindLogLine)
	b.Publish(e)

	select {
	case got := <-ch:
		assert.Equal(t, events.KindLogLine, got.Kind)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("event not delivered within 50ms")
	}
}

func TestBus_DeliversEvent_ToMultipleSubscribers(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	const n = 5
	chans := make([]<-chan events.Event, n)
	for i := range chans {
		ch, _ := b.Subscribe()
		chans[i] = ch
	}

	b.Publish(newTestEvent(events.KindProcessStarted))

	for i, ch := range chans {
		select {
		case got := <-ch:
			assert.Equal(t, events.KindProcessStarted, got.Kind, "subscriber %d", i)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d did not receive event within 100ms", i)
		}
	}
}

func TestBus_DoesNotBlock_WhenNoSubscribers(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	done := make(chan struct{})
	go func() {
		b.Publish(newTestEvent(events.KindLogLine))
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(10 * time.Millisecond):
		t.Fatal("Publish blocked with no subscribers")
	}
}

func TestBus_DoesNotBlock_WhenSubscriberChannelIsFull(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	// Subscribe but never read
	b.Subscribe()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 300; i++ {
			b.Publish(newTestEvent(events.KindLogLine))
		}
		close(done)
	}()

	select {
	case <-done:
		// all publishes returned immediately
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Publish blocked when subscriber channel is full")
	}
}

func TestBus_DropsOldestEvent_WhenBusBufferFull(t *testing.T) {
	// Block the fanOut goroutine while we fill the bus buffer.
	// This simulates the bus buffer being overwhelmed by fast publishers.
	var blocked atomic.Bool
	blocked.Store(true)
	blocker := make(chan struct{})
	events.SetFanOutHook(func() {
		if blocked.Load() {
			<-blocker
		}
	})
	t.Cleanup(func() { events.SetFanOutHook(nil) })

	b := events.NewBus()
	defer b.Close()

	b.Subscribe()

	// Publish more events than the bus buffer (4096) while fanOut is blocked.
	for i := 0; i < 5000; i++ {
		b.Publish(newTestEvent(events.KindLogLine))
	}

	// Unblock fanOut
	blocked.Store(false)
	close(blocker)

	assert.Greater(t, b.DroppedCount(), int64(0), "DroppedCount should be > 0 when bus buffer overflows")
}

func TestBus_UnsubscribeStops_DeliveryToThatSubscriber(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	ch, unsub := b.Subscribe()
	unsub()

	// Channel should be closed after unsub
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after unsub")
	case <-time.After(50 * time.Millisecond):
		t.Fatal("channel not closed within 50ms after unsub")
	}

	// Publishing after unsub should not panic or deliver to the unsubscribed channel
	b.Publish(events.NewEvent(events.KindLogLine, nil))
}

func TestBus_Close_ClosesAllSubscriberChannels(t *testing.T) {
	b := events.NewBus()

	ch, _ := b.Subscribe()
	b.Close()

	// Channel should be closed; receive returns zero value + false
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel not closed within 100ms after bus.Close()")
	}
}

func TestBus_IsConcurrently_Safe(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	const goroutines = 10
	var wg sync.WaitGroup

	// 10 publishers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.Publish(newTestEvent(events.KindLogLine))
			}
		}()
	}

	// 10 subscribers
	for i := 0; i < goroutines; i++ {
		ch, unsub := b.Subscribe()
		wg.Add(1)
		go func(ch <-chan events.Event, unsub func()) {
			defer wg.Done()
			defer unsub()
			timeout := time.After(2 * time.Second)
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-timeout:
					return
				}
			}
		}(ch, unsub)
	}

	wg.Wait()
}

func TestBus_PreservesOrder_FromSinglePublisher(t *testing.T) {
	b := events.NewBus()
	defer b.Close()

	ch, _ := b.Subscribe()

	const n = 100
	done := make(chan struct{})
	received := make([]events.Event, 0, n)

	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			select {
			case e := <-ch:
				received = append(received, e)
			case <-time.After(500 * time.Millisecond):
				return
			}
		}
	}()

	kinds := make([]events.Kind, n)
	for i := 0; i < n; i++ {
		k := events.KindLogLine
		if i%2 == 0 {
			k = events.KindProcessStarted
		}
		kinds[i] = k
		b.Publish(events.NewEvent(k, nil))
	}

	<-done

	require.Len(t, received, n)
	for i, e := range received {
		assert.Equal(t, kinds[i], e.Kind, "event %d: wrong kind", i)
	}
}

func TestBus_NoGoroutineLeak_AfterClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	b := events.NewBus()
	ch, _ := b.Subscribe()
	b.Close()

	// Drain closed channel
	for range ch {
	}
}
