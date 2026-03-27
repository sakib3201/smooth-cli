package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/smoothcli/smooth-cli/internal/api"
	"github.com/smoothcli/smooth-cli/internal/attention"
	"github.com/smoothcli/smooth-cli/internal/config"
	"github.com/smoothcli/smooth-cli/internal/events"
	"github.com/smoothcli/smooth-cli/internal/logstore"
	"github.com/smoothcli/smooth-cli/internal/store"
	"github.com/smoothcli/smooth-cli/internal/supervisor"
	"github.com/smoothcli/smooth-cli/internal/tui"
)

func init() {
	rootCmd.RunE = run
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig(configFlag)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	home, _ := os.UserHomeDir()
	projectDir := filepath.Join(home, ".smooth", cfg.Project)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	stateDB, err := store.OpenDB(filepath.Join(projectDir, "state.db"))
	if err != nil {
		return fmt.Errorf("state db: %w", err)
	}
	defer stateDB.Close()

	stateStore := store.New(stateDB)

	logDB, err := logstore.NewSQLiteStore(
		filepath.Join(projectDir, "logs.db"),
		cfg.Settings.LogBufferLines,
	)
	if err != nil {
		return fmt.Errorf("log db: %w", err)
	}
	defer logDB.Close()

	bus := events.NewBus()

	detector, err := attention.New()
	if err != nil {
		return fmt.Errorf("attention detector: %w", err)
	}

	var sup supervisor.Supervisor = supervisor.New(bus, logDB, detector)

	for _, pc := range cfg.Processes {
		if err := sup.Register(pc); err != nil {
			return fmt.Errorf("register %s: %w", pc.Name, err)
		}
	}

	apiDeps := api.Deps{
		Supervisor: &supervisorAdapter{sup: sup},
		LogStore:   &logStoreAdapter{store: logDB},
		State:      &stateAdapter{store: stateStore},
		Config:     &configAdapter{cfg: cfg},
		Bus:        bus,
	}

	apiServer := api.New(portFlag, apiDeps)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if portFlag != 0 {
			apiServer.Start(ctx)
		}
	}()

	go func() {
		ch, unsub := bus.Subscribe()
		defer unsub()
		for ev := range ch {
			if ev.Kind == events.KindProcessStarted {
				if pe, ok := ev.Payload().(events.ProcessEvent); ok {
					stateStore.SetProcess(pe.State)
				}
			}
		}
	}()

	tuiModel := tui.New()
	tuiModel = tuiModel.InitProcesses(sup.States())

	program := tea.NewProgram(tuiModel)
	if err := program.Start(); err != nil {
		cancel()
		return err
	}

	sup.Close(ctx)
	bus.Close()

	return nil
}

type supervisorAdapter struct {
	sup supervisor.Supervisor
}

func (a *supervisorAdapter) State(name string) (map[string]interface{}, error) {
	s, err := a.sup.State(name)
	return stateToMap(s), err
}

func (a *supervisorAdapter) States() map[string]map[string]interface{} {
	states := a.sup.States()
	result := make(map[string]map[string]interface{})
	for k, v := range states {
		result[k] = stateToMap(v)
	}
	return result
}

func (a *supervisorAdapter) Start(ctx context.Context, name string) error {
	return a.sup.Start(ctx, name)
}

func (a *supervisorAdapter) Stop(ctx context.Context, name string) error {
	return a.sup.Stop(ctx, name)
}

func (a *supervisorAdapter) Restart(ctx context.Context, name string) error {
	return a.sup.Restart(ctx, name)
}

func (a *supervisorAdapter) SendInput(ctx context.Context, name string, input []byte) error {
	return a.sup.SendInput(ctx, name, input)
}

type logStoreAdapter struct {
	store logstore.Store
}

func (a *logStoreAdapter) Lines(process string, n int) []interface{} {
	lines := a.store.Lines(process, n)
	result := make([]interface{}, len(lines))
	for i, l := range lines {
		result[i] = l
	}
	return result
}

type stateAdapter struct {
	store store.Store
}

func (a *stateAdapter) AllProcesses() map[string]map[string]interface{} {
	states := a.store.AllProcesses()
	result := make(map[string]map[string]interface{})
	for k, v := range states {
		result[k] = stateToMap(v)
	}
	return result
}

type configAdapter struct {
	cfg *config.SmoothConfig
}

func (a *configAdapter) GetConfig() interface{} {
	return a.cfg
}

func stateToMap(s interface{}) map[string]interface{} {
	return nil
}

func loadConfig(path string) (*config.SmoothConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if filepath.Ext(path) == ".toml" {
		return config.ParseTOML(data)
	}
	return config.ParseYAML(data)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
