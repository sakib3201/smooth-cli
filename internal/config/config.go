package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Sentinel errors
var (
	ErrNotFound       = errors.New("config file not found")
	ErrInvalid        = errors.New("invalid config")
	ErrEmptyCommand   = errors.New("command must not be empty")
	ErrNullByte       = errors.New("command contains null byte")
	ErrCommandTooLong = errors.New("command exceeds 4096 characters")
	ErrCwdTraversal   = errors.New("cwd escapes project root")
)

// SmoothConfig is the parsed, validated configuration.
type SmoothConfig struct {
	Version   int
	Project   string
	Processes map[string]ProcessConfig
	Settings  Settings
}

// ProcessConfig holds the runtime configuration for one managed process.
type ProcessConfig struct {
	Name         string
	Command      string
	Args         []string
	Cwd          string
	Env          map[string]string
	AutoRestart  bool
	MaxRestarts  int
	RestartDelay time.Duration
	Group        string
	Attention    AttentionConfig
	// WatchPaths deferred to v2
}

// AttentionConfig holds additional regex patterns for one process.
type AttentionConfig struct {
	Patterns []string
}

// Settings holds global smooth settings.
type Settings struct {
	LogBufferLines   int
	PersistLogs      bool
	Notifications    bool
	PermissionGating bool
	MCP              MCPSettings
}

// MCPSettings holds MCP server settings.
type MCPSettings struct {
	Enabled        bool
	DangerousTools bool
}

// DiffResult describes what changed between two configs.
type DiffResult struct {
	Added    []string      // process names added
	Removed  []string      // process names removed
	Modified []ProcessDiff // fields that changed
}

// ProcessDiff is one field change on one process.
type ProcessDiff struct {
	Name  string
	Field string
	Old   string
	New   string
}

// rawConfig mirrors SmoothConfig with yaml/toml struct tags for unmarshalling.
type rawConfig struct {
	Version   int                   `yaml:"version"   toml:"version"`
	Project   string                `yaml:"project"   toml:"project"`
	Processes map[string]rawProcess `yaml:"processes" toml:"processes"`
	Settings  rawSettings           `yaml:"settings"  toml:"settings"`
}

type rawProcess struct {
	Command      string            `yaml:"command"       toml:"command"`
	Cwd          string            `yaml:"cwd"           toml:"cwd"`
	Env          map[string]string `yaml:"env"           toml:"env"`
	AutoRestart  bool              `yaml:"auto_restart"  toml:"auto_restart"`
	MaxRestarts  int               `yaml:"max_restarts"  toml:"max_restarts"`
	RestartDelay string            `yaml:"restart_delay" toml:"restart_delay"`
	Group        string            `yaml:"group"         toml:"group"`
	Attention    rawAttention      `yaml:"attention"     toml:"attention"`
}

type rawAttention struct {
	Patterns []string `yaml:"patterns" toml:"patterns"`
}

type rawSettings struct {
	LogBufferLines   int    `yaml:"log_buffer_lines"   toml:"log_buffer_lines"`
	PersistLogs      *bool  `yaml:"persist_logs"       toml:"persist_logs"`
	Notifications    *bool  `yaml:"notifications"      toml:"notifications"`
	PermissionGating *bool  `yaml:"permission_gating"  toml:"permission_gating"`
	MCP              rawMCP `yaml:"mcp"                toml:"mcp"`
}

type rawMCP struct {
	Enabled        *bool `yaml:"enabled"         toml:"enabled"`
	DangerousTools bool  `yaml:"dangerous_tools" toml:"dangerous_tools"`
}

// ParseYAML parses and validates a YAML config.
func ParseYAML(data []byte) (*SmoothConfig, error) {
	// Pre-scan for null bytes in command fields so we can return ErrNullByte
	// before the YAML parser rejects them as control characters.
	if err := preCheckNullBytes(data); err != nil {
		return nil, err
	}
	raw := &rawConfig{}
	if err := yaml.Unmarshal(data, raw); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err)
	}
	return validate(raw)
}

// preCheckNullBytes does a raw scan of the data for null byte sequences
// in command fields. This is necessary because YAML parsers reject null bytes
// before our field-level validation can run.
func preCheckNullBytes(data []byte) error {
	s := string(data)
	lines := strings.Split(s, "\n")
	inCommand := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "command:") {
			inCommand = true
			val := strings.TrimPrefix(trimmed, "command:")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, "\"'")
			if strings.ContainsRune(val, 0) {
				return fmt.Errorf("%w", ErrNullByte)
			}
		} else if inCommand && (strings.HasPrefix(trimmed, "-") || strings.Contains(line, ":")) {
			inCommand = false
		}
	}
	// Also check the raw bytes directly
	for _, b := range data {
		if b == 0 {
			return fmt.Errorf("%w", ErrNullByte)
		}
	}
	return nil
}

// ParseTOML parses and validates a TOML config.
func ParseTOML(data []byte) (*SmoothConfig, error) {
	raw := &rawConfig{}
	if _, err := toml.Decode(string(data), raw); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err)
	}
	return validate(raw)
}

func validate(raw *rawConfig) (*SmoothConfig, error) {
	if raw.Version != 1 {
		return nil, fmt.Errorf("%w: version must be 1, got %d", ErrInvalid, raw.Version)
	}
	if len(raw.Processes) == 0 {
		return nil, fmt.Errorf("%w: processes map must not be empty", ErrInvalid)
	}

	cfg := &SmoothConfig{
		Version:   raw.Version,
		Project:   raw.Project,
		Processes: make(map[string]ProcessConfig, len(raw.Processes)),
	}
	if cfg.Project == "" {
		cwd, _ := os.Getwd()
		cfg.Project = filepath.Base(cwd)
	}

	for name, rp := range raw.Processes {
		if strings.Contains(name, "/") {
			return nil, fmt.Errorf("%w: process name %q must not contain /", ErrInvalid, name)
		}
		cmd := interpolateEnv(strings.TrimSpace(rp.Command))
		if cmd == "" {
			return nil, fmt.Errorf("%w: process %q", ErrEmptyCommand, name)
		}
		if strings.ContainsRune(cmd, 0) {
			return nil, fmt.Errorf("%w: process %q", ErrNullByte, name)
		}
		if !utf8.ValidString(cmd) || len(cmd) > 4096 {
			return nil, fmt.Errorf("%w: process %q", ErrCommandTooLong, name)
		}

		delay := 2 * time.Second
		if rp.RestartDelay != "" {
			d, err := time.ParseDuration(rp.RestartDelay)
			if err != nil {
				return nil, fmt.Errorf("%w: process %q restart_delay: %s", ErrInvalid, name, err)
			}
			delay = d
		}

		maxR := rp.MaxRestarts
		if maxR == 0 {
			maxR = 5
		}

		group := rp.Group
		if group == "" {
			group = "default"
		}

		cwd := interpolateEnv(rp.Cwd)
		if cwd == "" {
			cwd = "."
		}

		env := make(map[string]string, len(rp.Env))
		for k, v := range rp.Env {
			env[k] = interpolateEnv(v)
		}

		parts := strings.Fields(cmd)
		cfg.Processes[name] = ProcessConfig{
			Name:         name,
			Command:      parts[0],
			Args:         parts[1:],
			Cwd:          cwd,
			Env:          env,
			AutoRestart:  rp.AutoRestart,
			MaxRestarts:  maxR,
			RestartDelay: delay,
			Group:        group,
			Attention:    AttentionConfig{Patterns: rp.Attention.Patterns},
		}
	}

	cfg.Settings = applySettingsDefaults(raw.Settings)
	return cfg, nil
}

func interpolateEnv(s string) string {
	return os.ExpandEnv(s)
}

func applySettingsDefaults(r rawSettings) Settings {
	s := Settings{
		LogBufferLines:   10000,
		PersistLogs:      true,
		Notifications:    true,
		PermissionGating: true,
		MCP: MCPSettings{
			Enabled:        true,
			DangerousTools: r.MCP.DangerousTools,
		},
	}
	if r.LogBufferLines > 0 {
		s.LogBufferLines = r.LogBufferLines
	}
	if r.PersistLogs != nil {
		s.PersistLogs = *r.PersistLogs
	}
	if r.Notifications != nil {
		s.Notifications = *r.Notifications
	}
	if r.PermissionGating != nil {
		s.PermissionGating = *r.PermissionGating
	}
	if r.MCP.Enabled != nil {
		s.MCP.Enabled = *r.MCP.Enabled
	}
	return s
}

// Diff computes what changed between two configs.
func Diff(old, new *SmoothConfig) DiffResult {
	d := DiffResult{}
	for name := range new.Processes {
		if _, exists := old.Processes[name]; !exists {
			d.Added = append(d.Added, name)
		}
	}
	for name := range old.Processes {
		if _, exists := new.Processes[name]; !exists {
			d.Removed = append(d.Removed, name)
		}
	}
	for name, np := range new.Processes {
		op, exists := old.Processes[name]
		if !exists {
			continue
		}
		if op.Command != np.Command {
			d.Modified = append(d.Modified, ProcessDiff{Name: name, Field: "command", Old: op.Command, New: np.Command})
		}
		if op.Cwd != np.Cwd {
			d.Modified = append(d.Modified, ProcessDiff{Name: name, Field: "cwd", Old: op.Cwd, New: np.Cwd})
		}
	}
	return d
}
