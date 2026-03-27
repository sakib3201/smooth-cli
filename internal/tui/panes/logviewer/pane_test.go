package logviewer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestPane_AutoScrolls_ToBottom_InFollowMode(t *testing.T) {
	m := New()
	m.Height = 10

	for i := 0; i < 20; i++ {
		m = m.Append(domain.LogLine{
			Process:   "test",
			Stream:    domain.Stdout,
			Timestamp: time.Now(),
			Raw:       []byte("line"),
			Stripped:  "line",
			Seq:       int64(i),
		})
	}

	assert.Equal(t, 10, m.Offset)
}

func TestPane_DoesNotAutoScroll_WhenFollowDisabled(t *testing.T) {
	m := New()
	m.Height = 10
	m.Follow = false

	for i := 0; i < 20; i++ {
		m = m.Append(domain.LogLine{
			Process:   "test",
			Stream:    domain.Stdout,
			Timestamp: time.Now(),
			Raw:       []byte("line"),
			Stripped:  "line",
			Seq:       int64(i),
		})
	}

	assert.Equal(t, 0, m.Offset)
}
