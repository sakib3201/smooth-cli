package supervisor

import (
	"errors"
	"sync"
	"time"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

var (
	ErrAlreadyRunning  = errors.New("process already running")
	ErrProcessNotFound = errors.New("process not found")
	ErrNotRunning      = errors.New("process not running")
)

type process struct {
	mu           sync.Mutex
	cfg          processConfig
	status       domain.ProcessStatus
	pid          int
	exitCode     *int
	startedAt    *time.Time
	crashedAt    *time.Time
	restartCount int
	cpuPercent   float64
	memoryMB     float64
	attention    bool
}

type processConfig struct {
	name         string
	command      string
	args         []string
	cwd          string
	env          []string
	autoRestart  bool
	maxRestarts  int
	restartDelay time.Duration
	group        string
}

func (p *process) snapshot() domain.ProcessState {
	p.mu.Lock()
	defer p.mu.Unlock()
	var uptime int64
	if p.startedAt != nil {
		uptime = time.Since(*p.startedAt).Milliseconds()
	}
	return domain.ProcessState{
		Name:         p.cfg.name,
		Command:      p.cfg.command,
		Args:         p.cfg.args,
		Cwd:          p.cfg.cwd,
		Group:        p.cfg.group,
		Status:       p.status,
		PID:          p.pid,
		ExitCode:     p.exitCode,
		StartedAt:    p.startedAt,
		CrashedAt:    p.crashedAt,
		UptimeMs:     uptime,
		RestartCount: p.restartCount,
		CPUPercent:   p.cpuPercent,
		MemoryMB:     p.memoryMB,
		Attention:    p.attention,
	}
}

func canTransition(from, to domain.ProcessStatus) bool {
	allowed := map[domain.ProcessStatus][]domain.ProcessStatus{
		domain.StatusStopped:    {domain.StatusStarting},
		domain.StatusStarting:   {domain.StatusRunning, domain.StatusCrashed, domain.StatusStopped},
		domain.StatusRunning:    {domain.StatusStopped, domain.StatusCrashed, domain.StatusRestarting},
		domain.StatusCrashed:    {domain.StatusRestarting, domain.StatusStopped},
		domain.StatusRestarting: {domain.StatusStarting, domain.StatusStopped},
	}
	for _, s := range allowed[from] {
		if s == to {
			return true
		}
	}
	return false
}
