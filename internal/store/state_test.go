package store_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/store"
)

func validProcessState() domain.ProcessState {
	pid := 1234
	return domain.ProcessState{
		Name:         "test-proc",
		Command:      "echo hello",
		Args:         []string{"hello"},
		Cwd:          ".",
		Status:       domain.StatusRunning,
		PID:          pid,
		ExitCode:     nil,
		StartedAt:    ptr(time.Now()),
		RestartCount: 0,
		CPUPercent:   0.5,
		MemoryMB:     10.5,
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestStore_SetAndGetProcess_RoundTrips(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	state := validProcessState()
	s.SetProcess(state)

	got, err := s.GetProcess("test-proc")
	require.NoError(t, err)
	assert.Equal(t, state.Name, got.Name)
	assert.Equal(t, state.Status, got.Status)
	assert.Equal(t, state.PID, got.PID)
}

func TestStore_GetProcess_ReturnsError_WhenNotFound(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	_, err := s.GetProcess("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrProcessNotFound))
}

func TestStore_AllProcesses_ReturnsSnapshot_NotReference(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.SetProcess(validProcessState())

	snap1 := s.AllProcesses()
	for k := range snap1 {
		v := snap1[k]
		v.Name = "mutated"
		snap1[k] = v
	}

	snap2 := s.AllProcesses()
	assert.Equal(t, "test-proc", snap2["test-proc"].Name)
}

func TestStore_SetProcess_UpdatesExistingEntry(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.SetProcess(validProcessState())
	s.SetProcess(domain.ProcessState{Name: "test-proc", Status: domain.StatusCrashed})

	got, err := s.GetProcess("test-proc")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCrashed, got.Status)
}

func TestStore_AddAttentionEvent_AppearsInActiveEvents(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.AddAttentionEvent(validAttentionEvent())

	active := s.ActiveAttentionEvents()
	require.Len(t, active, 1)
	assert.Equal(t, "test-proc", active[0].Process)
}

func TestStore_ResolveAttention_RemovesFromActiveEvents(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.AddAttentionEvent(validAttentionEvent())
	s.ResolveAttention("test-proc")

	active := s.ActiveAttentionEvents()
	require.Len(t, active, 0)
}

func TestStore_ActiveAttentionEvents_OrderedNewestFirst(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	e1 := validAttentionEvent()
	e1.Timestamp = time.Now().Add(-2 * time.Hour)
	e2 := validAttentionEvent()
	e2.Timestamp = time.Now().Add(-1 * time.Hour)
	e3 := validAttentionEvent()
	e3.Timestamp = time.Now()

	s.AddAttentionEvent(e1)
	s.AddAttentionEvent(e2)
	s.AddAttentionEvent(e3)

	active := s.ActiveAttentionEvents()
	require.Len(t, active, 3)
	assert.True(t, active[0].Timestamp.After(active[2].Timestamp))
}

func TestStore_AddPermissionRequest_AppearsInPending(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.AddPermissionRequest(validPermissionReq())

	pending := s.PendingPermissionRequests()
	require.Len(t, pending, 1)
	assert.Equal(t, "test-uuid", pending[0].ID)
}

func TestStore_ResolvePermissionRequest_Approve_RemovesFromPending(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.AddPermissionRequest(validPermissionReq())
	err := s.ResolvePermissionRequest("test-uuid", true)
	require.NoError(t, err)

	pending := s.PendingPermissionRequests()
	require.Len(t, pending, 0)
}

func TestStore_ResolvePermissionRequest_Deny_RemovesFromPending(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	s.AddPermissionRequest(validPermissionReq())
	err := s.ResolvePermissionRequest("test-uuid", false)
	require.NoError(t, err)

	pending := s.PendingPermissionRequests()
	require.Len(t, pending, 0)
}

func TestStore_ResolvePermissionRequest_ReturnsError_WhenIDNotFound(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	err := s.ResolvePermissionRequest("nonexistent", true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrRequestNotFound))
}

func TestStore_GetSetSetting_RoundTrips(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	err := s.SetSetting("theme", "dark")
	require.NoError(t, err)

	val, err := s.GetSetting("theme")
	require.NoError(t, err)
	assert.Equal(t, "dark", val)
}

func TestStore_GetSetting_ReturnsError_WhenKeyNotFound(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	_, err := s.GetSetting("nonexistent")
	require.Error(t, err)
}

func TestStore_IsConcurrently_Safe(t *testing.T) {
	s := store.New(nil)
	defer s.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				name := "proc"
				state := domain.ProcessState{Name: name, Status: domain.StatusRunning}
				s.SetProcess(state)
				s.GetProcess(name)
				s.AllProcesses()
				s.AddAttentionEvent(validAttentionEvent())
			}
		}()
	}
	wg.Wait()
}

func validAttentionEvent() domain.AttentionEvent {
	return domain.AttentionEvent{
		Process:   "test-proc",
		Pattern:   "test-pattern",
		Context:   "test context",
		Timestamp: time.Now(),
		Resolved:  false,
	}
}

func validPermissionReq() domain.PermissionRequest {
	return domain.PermissionRequest{
		ID:          "test-uuid",
		Process:     "test-proc",
		Action:      "new_process",
		OldValue:    "",
		NewValue:    "echo hello",
		RequestedAt: time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
}
