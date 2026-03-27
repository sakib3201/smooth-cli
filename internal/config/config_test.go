package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/smoothcli/smooth-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAML_ValidMinimal(t *testing.T) {
	data := []byte(`version: 1
processes:
  api:
    command: "echo hello"
`)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 1, cfg.Version)
	assert.Contains(t, cfg.Processes, "api")
	assert.Equal(t, "echo", cfg.Processes["api"].Command)
}

func TestParseYAML_RejectsEmptyCommand(t *testing.T) {
	data, err := os.ReadFile("../../testdata/fixtures/invalid_missing_cmd.yml")
	require.NoError(t, err)
	_, err = config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrEmptyCommand)
}

func TestParseYAML_RejectsNullByte(t *testing.T) {
	data := []byte("version: 1\nprocesses:\n  api:\n    command: \"go\x00run\"\n")
	_, err := config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrNullByte)
}

func TestParseYAML_ExpandsEnvVars(t *testing.T) {
	t.Setenv("TEST_PORT", "3000")
	data, err := os.ReadFile("../../testdata/fixtures/env_interpolation.yml")
	require.NoError(t, err)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	// command is "echo 3000" after interpolation, split into command=echo args=[3000]
	assert.Equal(t, "echo", cfg.Processes["api"].Command)
	assert.Equal(t, []string{"3000"}, cfg.Processes["api"].Args)
}

func TestParseYAML_DefaultsApplied(t *testing.T) {
	data := []byte(`version: 1
processes:
  api:
    command: "echo hi"
`)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	p := cfg.Processes["api"]
	assert.Equal(t, 5, p.MaxRestarts)
	assert.Equal(t, 2*time.Second, p.RestartDelay)
	assert.Equal(t, "default", p.Group)
	assert.True(t, cfg.Settings.PersistLogs)
	assert.True(t, cfg.Settings.Notifications)
	assert.True(t, cfg.Settings.PermissionGating)
	assert.True(t, cfg.Settings.MCP.Enabled)
	assert.Equal(t, 10000, cfg.Settings.LogBufferLines)
}

func TestParseYAML_RejectsVersionNot1(t *testing.T) {
	data := []byte("version: 2\nprocesses:\n  api:\n    command: echo\n")
	_, err := config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrInvalid)
}

func TestParseYAML_RejectsEmptyProcessMap(t *testing.T) {
	data := []byte("version: 1\nprocesses: {}\n")
	_, err := config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrInvalid)
}

func TestParseYAML_RejectsVersionMissing(t *testing.T) {
	data := []byte("processes:\n  api:\n    command: echo\n")
	_, err := config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrInvalid)
}

func TestParseYAML_RejectsProcessNameWithSlash(t *testing.T) {
	data := []byte("version: 1\nprocesses:\n  api/server:\n    command: echo\n")
	_, err := config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrInvalid)
}

func TestParseYAML_RejectsCommandExceeding4096Chars(t *testing.T) {
	longCmd := make([]byte, 4097)
	for i := range longCmd {
		longCmd[i] = 'a'
	}
	data := append([]byte("version: 1\nprocesses:\n  api:\n    command: \""), longCmd...)
	data = append(data, '"', '\n')
	_, err := config.ParseYAML(data)
	assert.ErrorIs(t, err, config.ErrCommandTooLong)
}

func TestParseYAML_SetsProjectName_ToBasenameOfCwd_WhenOmitted(t *testing.T) {
	data := []byte(`version: 1
processes:
  api:
    command: "echo hello"
`)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Project)
}

func TestParseYAML_PopulatesProcessName_FromMapKey(t *testing.T) {
	data := []byte(`version: 1
processes:
  api-server:
    command: "echo hello"
`)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	assert.Equal(t, "api-server", cfg.Processes["api-server"].Name)
}

func TestParseYAML_MergesSettings_WithDefaults(t *testing.T) {
	data := []byte(`version: 1
processes:
  api:
    command: "echo hello"
settings:
  notifications: false
`)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	assert.False(t, cfg.Settings.Notifications)
	assert.Equal(t, 10000, cfg.Settings.LogBufferLines)
	assert.True(t, cfg.Settings.PersistLogs)
}

func TestParseTOML_AcceptsValidConfig(t *testing.T) {
	data := []byte(`version = 1
[processes.api]
command = "echo hello"
`)
	cfg, err := config.ParseTOML(data)
	require.NoError(t, err)
	assert.Contains(t, cfg.Processes, "api")
}

func TestDiff_DetectsAddedProcess(t *testing.T) {
	old := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "echo"},
	}}
	newCfg := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api":    {Command: "echo"},
		"worker": {Command: "node worker.js"},
	}}
	d := config.Diff(old, newCfg)
	assert.Equal(t, []string{"worker"}, d.Added)
	assert.Empty(t, d.Removed)
	assert.Empty(t, d.Modified)
}

func TestDiff_DetectsRemovedProcess(t *testing.T) {
	old := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api":    {Command: "echo"},
		"worker": {Command: "node"},
	}}
	newCfg := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "echo"},
	}}
	d := config.Diff(old, newCfg)
	assert.Equal(t, []string{"worker"}, d.Removed)
	assert.Empty(t, d.Added)
}

func TestDiff_DetectsCommandChange(t *testing.T) {
	old := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "node", Cwd: "."},
	}}
	newCfg := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "deno", Cwd: "."},
	}}
	d := config.Diff(old, newCfg)
	require.Len(t, d.Modified, 1)
	assert.Equal(t, "command", d.Modified[0].Field)
	assert.Equal(t, "node", d.Modified[0].Old)
	assert.Equal(t, "deno", d.Modified[0].New)
}

func TestDiff_DetectsCwdChange(t *testing.T) {
	old := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "echo", Cwd: "./old"},
	}}
	newCfg := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "echo", Cwd: "./new"},
	}}
	d := config.Diff(old, newCfg)
	require.Len(t, d.Modified, 1)
	assert.Equal(t, "cwd", d.Modified[0].Field)
}

func TestDiff_ReturnsEmptyDiff_WhenConfigsAreIdentical(t *testing.T) {
	old := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "echo", Cwd: "."},
	}}
	newCfg := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api": {Command: "echo", Cwd: "."},
	}}
	d := config.Diff(old, newCfg)
	assert.Empty(t, d.Added)
	assert.Empty(t, d.Removed)
	assert.Empty(t, d.Modified)
}

func TestParseYAML_InterpolatesEnvVars_InCwdField(t *testing.T) {
	t.Setenv("MY_CWD", "/tmp/project")
	data := []byte(`version: 1
processes:
  api:
    command: "echo hello"
    cwd: "${MY_CWD}"
`)
	cfg, err := config.ParseYAML(data)
	require.NoError(t, err)
	assert.Equal(t, "/tmp/project", cfg.Processes["api"].Cwd)
}

func TestDiff_IsOrderIndependent(t *testing.T) {
	old := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api":    {Command: "echo"},
		"worker": {Command: "node"},
	}}
	new := &config.SmoothConfig{Processes: map[string]config.ProcessConfig{
		"api":      {Command: "echo"},
		"frontend": {Command: "npm run dev"},
	}}
	d := config.Diff(old, new)
	assert.ElementsMatch(t, []string{"frontend"}, d.Added)
	assert.ElementsMatch(t, []string{"worker"}, d.Removed)
	assert.Empty(t, d.Modified)
}
