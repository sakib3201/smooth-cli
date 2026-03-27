package domain

import "time"

// ProcessStatus represents the lifecycle state of a managed process.
// All transitions must be through the state machine defined in supervisor.
type ProcessStatus string

const (
	StatusStarting   ProcessStatus = "starting"
	StatusRunning    ProcessStatus = "running"
	StatusStopped    ProcessStatus = "stopped"   // manual stop
	StatusCrashed    ProcessStatus = "crashed"   // exited non-zero or signalled
	StatusRestarting ProcessStatus = "restarting"
)

// ProcessState is the full runtime snapshot of one managed process.
// It is immutable once constructed — callers receive copies, not pointers.
type ProcessState struct {
	Name         string
	Command      string
	Args         []string
	Cwd          string
	Env          map[string]string
	Group        string
	Status       ProcessStatus
	PID          int           // 0 if not running
	ExitCode     *int          // nil if not exited
	StartedAt    *time.Time
	CrashedAt    *time.Time
	UptimeMs     int64
	RestartCount int
	CPUPercent   float64
	MemoryMB     float64
	Attention    bool          // true if attention detector has flagged this process
}

// LogLine is one line of output from a managed process.
type LogLine struct {
	Process   string
	Stream    Stream        // Stdout or Stderr
	Timestamp time.Time
	Raw       []byte        // ANSI escape codes preserved — for xterm.js / TUI rendering
	Stripped  string        // ANSI removed — for search and attention detection
	Seq       int64         // monotonically increasing per process, for ordering
}

// Stream identifies which output stream a log line came from.
type Stream string

const (
	Stdout Stream = "stdout"
	Stderr Stream = "stderr"
)

// AttentionEvent signals that a process needs user input or attention.
type AttentionEvent struct {
	Process   string
	Pattern   string        // the regex or OSC sequence that matched
	Context   string        // the log line that triggered the event (stripped)
	Timestamp time.Time
	Resolved  bool          // set to true when user acknowledges
}

// PermissionRequest is raised when a config change requires user consent.
type PermissionRequest struct {
	ID          string        // UUID for correlation
	Process     string        // process being affected
	Action      string        // "new_process" | "command_changed" | "cwd_changed"
	OldValue    string        // previous value (empty for new processes)
	NewValue    string        // proposed new value
	RequestedAt time.Time
	ExpiresAt   time.Time     // auto-denies after this time
}

// PermissionDecision records the outcome of a PermissionRequest.
type PermissionDecision struct {
	RequestID string
	Approved  bool
	DecidedAt time.Time
}
