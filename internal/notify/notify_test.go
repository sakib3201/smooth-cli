package notify_test

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/notify"
)

func TestMockNotifier_RecordsCallCount(t *testing.T) {
	mock := notify.NewMock()

	mock.Notify("Title A", "Body A")
	mock.Notify("Title B", "Body B")

	assert.Equal(t, 2, mock.CallCount())
}

func TestMockNotifier_RecordsCallArguments(t *testing.T) {
	mock := notify.NewMock()

	mock.Notify("Title A", "Body A")
	mock.Notify("Title B", "Body B")

	calls := mock.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, "Title A", calls[0].Title)
	assert.Equal(t, "Body A", calls[0].Body)
	assert.Equal(t, "Title B", calls[1].Title)
	assert.Equal(t, "Body B", calls[1].Body)
}

func TestMockNotifier_IsConcurrently_Safe(t *testing.T) {
	mock := notify.NewMock()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			mock.Notify("Title", "Body")
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 20, mock.CallCount())
}

func TestBeeepNotifier_DoesNotPanic_WhenDisplayUnavailable(t *testing.T) {
	if os.Getenv("DISPLAY") != "" {
		t.Skip("skipping in headless CI environment")
	}

	n := notify.New()
	err := n.Notify("Test", "Body")
	if err != nil {
		t.Logf("beeep returned error (expected in headless env): %v", err)
	}
}
