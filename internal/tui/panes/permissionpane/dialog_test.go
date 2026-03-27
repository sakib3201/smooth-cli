package permissionpane

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestDialog_ShowsRequests_WhenPresent(t *testing.T) {
	m := New()
	m = m.AddRequest(domain.PermissionRequest{
		ID:          "test-id",
		Process:     "test-proc",
		Action:      "new_process",
		NewValue:    "echo hello",
		RequestedAt: time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})

	assert.True(t, m.Visible)
	assert.Len(t, m.Requests, 1)
}

func TestDialog_Hides_WhenAllResolved(t *testing.T) {
	m := New()
	m = m.AddRequest(domain.PermissionRequest{
		ID:          "test-id",
		Process:     "test-proc",
		Action:      "new_process",
		NewValue:    "echo hello",
		RequestedAt: time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})
	m = m.ApproveCurrent()

	assert.False(t, m.Visible)
	assert.Len(t, m.Requests, 0)
}
