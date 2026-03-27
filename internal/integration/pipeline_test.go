//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/attention"
	"github.com/smoothcli/smooth-cli/internal/config"
	"github.com/smoothcli/smooth-cli/internal/events"
	"github.com/smoothcli/smooth-cli/internal/logstore"
	"github.com/smoothcli/smooth-cli/internal/store"
	"github.com/smoothcli/smooth-cli/internal/supervisor"
)

func TestPipeline_StartProcess_EmitsEvents(t *testing.T) {
	tmp := t.TempDir()

	cfgData := []byte(`
version: 1
processes:
  echo:
    command: echo hello
    cwd: "` + tmp + `"
`)
	cfg, err := config.ParseYAML(cfgData)
	require.NoError(t, err)

	stateDB, err := store.OpenDB(filepath.Join(tmp, "state.db"))
	require.NoError(t, err)
	defer stateDB.Close()

	logDB, err := logstore.NewSQLiteStore(filepath.Join(tmp, "logs.db"), 1000)
	require.NoError(t, err)
	defer logDB.Close()

	bus := events.NewBus()
	defer bus.Close()

	detector, err := attention.New()
	require.NoError(t, err)

	sup := supervisor.New(bus, logDB, detector)

	err = sup.Register(cfg.Processes["echo"])
	require.NoError(t, err)

	ch, unsub := bus.Subscribe()
	defer unsub()

	err = sup.Start(context.Background(), "echo")
	require.NoError(t, err)

	var started bool
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		select {
		case ev := <-ch:
			if ev.Kind == events.KindProcessStarted {
				started = true
			}
		default:
		}
		if started {
			break
		}
	}
	assert.True(t, started, "KindProcessStarted event should have been emitted")

	states := sup.States()
	assert.Contains(t, states, "echo")
	assert.Equal(t, "running", string(states["echo"].Status))

	sup.Close(context.Background())
}

func TestPipeline_HotReload_EmitsConfigChanged(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "smooth.yml")

	err := os.WriteFile(configPath, []byte(`
version: 1
processes:
  echo:
    command: echo hello
    cwd: "`+tmp+`"
`), 0644)
	require.NoError(t, err)

	cfg, err := config.ParseYAML([]byte(`
version: 1
processes:
  echo:
    command: echo hello
    cwd: "` + tmp + `"
`))
	require.NoError(t, err)

	bus := events.NewBus()
	defer bus.Close()

	logDB, err := logstore.NewSQLiteStore(filepath.Join(tmp, "logs.db"), 1000)
	require.NoError(t, err)
	defer logDB.Close()

	detector, err := attention.New()
	require.NoError(t, err)

	sup := supervisor.New(bus, logDB, detector)

	err = sup.Register(cfg.Processes["echo"])
	require.NoError(t, err)

	ch, unsub := bus.Subscribe()
	defer unsub()

	watcher, err := config.Watch(context.Background(), configPath, cfg, func(newCfg *config.SmoothConfig, diff config.DiffResult) {}, make(chan error))
	require.NoError(t, err)
	defer watcher.Close()

	time.Sleep(200 * time.Millisecond)
}

func TestPipeline_MCP_HealthCheck(t *testing.T) {
	tmp := t.TempDir()

	cfgData := []byte(`
version: 1
processes:
  echo:
    command: echo hello
    cwd: "` + tmp + `"
`)
	cfg, err := config.ParseYAML(cfgData)
	require.NoError(t, err)

	bus := events.NewBus()
	defer bus.Close()

	logDB, err := logstore.NewSQLiteStore(filepath.Join(tmp, "logs.db"), 1000)
	require.NoError(t, err)
	defer logDB.Close()

	detector, err := attention.New()
	require.NoError(t, err)

	sup := supervisor.New(bus, logDB, detector)
	defer sup.Close(context.Background())

	err = sup.Register(cfg.Processes["echo"])
	require.NoError(t, err)
}
