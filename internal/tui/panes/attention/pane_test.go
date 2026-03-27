package attention

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestPane_ShowsEvents_WhenPresent(t *testing.T) {
	m := New()
	m = m.AddEvent(domain.AttentionEvent{
		Process:   "test",
		Pattern:   "test",
		Context:   "test context",
		Timestamp: time.Now(),
	})

	assert.True(t, m.Visible)
	assert.Len(t, m.Events, 1)
}

func TestPane_Hides_WhenAllResolved(t *testing.T) {
	m := New()
	m = m.AddEvent(domain.AttentionEvent{
		Process:   "test",
		Pattern:   "test",
		Context:   "test context",
		Timestamp: time.Now(),
	})
	m = m.Resolve("test")

	assert.False(t, m.Visible)
	assert.Len(t, m.Events, 0)
}
