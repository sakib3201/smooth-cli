package permission_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/events"
	"github.com/smoothcli/smooth-cli/internal/permission"
	"github.com/smoothcli/smooth-cli/internal/store"
)

type fakeBus struct {
	published []events.Event
}

func (f *fakeBus) Publish(e events.Event) {
	f.published = append(f.published, e)
}

func (f *fakeBus) Subscribe() (<-chan events.Event, func()) {
	ch := make(chan events.Event)
	return ch, func() { close(ch) }
}

func (f *fakeBus) DroppedCount() int64 { return 0 }

func (f *fakeBus) Close() {}

type fakeStore struct {
	permissions []domain.PermissionRequest
}

func (f *fakeStore) SetProcess(state domain.ProcessState) {}
func (f *fakeStore) GetProcess(name string) (domain.ProcessState, error) {
	return domain.ProcessState{}, nil
}
func (f *fakeStore) AllProcesses() map[string]domain.ProcessState   { return nil }
func (f *fakeStore) AddAttentionEvent(e domain.AttentionEvent)      {}
func (f *fakeStore) ResolveAttention(process string)                {}
func (f *fakeStore) ActiveAttentionEvents() []domain.AttentionEvent { return nil }
func (f *fakeStore) AddPermissionRequest(req domain.PermissionRequest) {
	f.permissions = append(f.permissions, req)
}
func (f *fakeStore) ResolvePermissionRequest(id string, approved bool) error {
	for i, r := range f.permissions {
		if r.ID == id {
			f.permissions = append(f.permissions[:i], f.permissions[i+1:]...)
			return nil
		}
	}
	return store.ErrRequestNotFound
}
func (f *fakeStore) PendingPermissionRequests() []domain.PermissionRequest { return nil }
func (f *fakeStore) GetSetting(key string) (string, error)                 { return "", nil }
func (f *fakeStore) SetSetting(key, value string) error                    { return nil }
func (f *fakeStore) Close() error                                          { return nil }

func TestGate_Record_ReturnsError_WhenIDNotFound(t *testing.T) {
	bus := &fakeBus{}
	s := &fakeStore{}
	g := permission.New(bus, s)

	err := g.Record("nonexistent", true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, permission.ErrRequestNotFound))
}
