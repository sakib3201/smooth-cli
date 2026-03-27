package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestWatch_CallsOnChange_WhenFileIsModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "smooth.yml")

	initial := []byte("version: 1\nprocesses:\n  alpha:\n    command: echo alpha\n")
	require.NoError(t, os.WriteFile(path, initial, 0o644))

	cfg, err := ParseYAML(initial)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 4)
	called := make(chan struct {
		cfg  *SmoothConfig
		diff DiffResult
	}, 1)

	err = Watch(ctx, path, cfg, func(newCfg *SmoothConfig, diff DiffResult) {
		select {
		case called <- struct {
			cfg  *SmoothConfig
			diff DiffResult
		}{newCfg, diff}:
		default:
		}
	}, errCh)
	require.NoError(t, err)

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	updated := []byte("version: 1\nprocesses:\n  alpha:\n    command: echo alpha\n  beta:\n    command: echo beta\n")
	require.NoError(t, os.WriteFile(path, updated, 0o644))

	select {
	case result := <-called:
		assert.Contains(t, result.cfg.Processes, "beta")
		assert.Contains(t, result.diff.Added, "beta")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("onChange not called within 500ms")
	}
}

func TestWatch_DoesNotCallOnChange_WhenNewConfigIsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "smooth.yml")

	initial := []byte("version: 1\nprocesses:\n  alpha:\n    command: echo alpha\n")
	require.NoError(t, os.WriteFile(path, initial, 0o644))

	cfg, err := ParseYAML(initial)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 4)
	called := make(chan struct{}, 1)

	err = Watch(ctx, path, cfg, func(newCfg *SmoothConfig, diff DiffResult) {
		select {
		case called <- struct{}{}:
		default:
		}
	}, errCh)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Invalid: empty command
	invalid := []byte("version: 1\nprocesses:\n  broken:\n    command: \"\"\n")
	require.NoError(t, os.WriteFile(path, invalid, 0o644))

	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("errCh did not receive error within 500ms")
	}

	// Ensure onChange was NOT called
	select {
	case <-called:
		t.Fatal("onChange should not have been called for invalid config")
	default:
	}
}

func TestWatch_StopsCleanly_WhenContextCancelled(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "smooth.yml")

	initial := []byte("version: 1\nprocesses:\n  alpha:\n    command: echo alpha\n")
	require.NoError(t, os.WriteFile(path, initial, 0o644))

	cfg, err := ParseYAML(initial)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 4)
	err = Watch(ctx, path, cfg, func(_ *SmoothConfig, _ DiffResult) {}, errCh)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	cancel()

	// Give goroutine time to exit — goleak will verify no leak
	time.Sleep(200 * time.Millisecond)
}
