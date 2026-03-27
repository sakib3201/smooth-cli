package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/smoothcli/smooth-cli/internal/attention"
	"github.com/smoothcli/smooth-cli/internal/config"
	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/events"
	"github.com/smoothcli/smooth-cli/internal/logstore"
)

type Supervisor interface {
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	SendInput(ctx context.Context, name string, input []byte) error
	State(name string) (domain.ProcessState, error)
	States() map[string]domain.ProcessState
	Register(cfg config.ProcessConfig) error
	Deregister(ctx context.Context, name string) error
	Close(ctx context.Context) error
}

type supervisor struct {
	mu        sync.RWMutex
	processes map[string]*process
	configs   map[string]processConfig
	ptys      map[string]*os.File
	bus       events.Bus
	logStore  logstore.Store
	detector  attention.Detector
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

func New(bus events.Bus, logStore logstore.Store, detector attention.Detector) *supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &supervisor{
		processes: make(map[string]*process),
		configs:   make(map[string]processConfig),
		ptys:      make(map[string]*os.File),
		bus:       bus,
		logStore:  logStore,
		detector:  detector,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *supervisor) Register(cfg config.ProcessConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.configs[cfg.Name]; exists {
		return fmt.Errorf("process %q already registered", cfg.Name)
	}

	env := make([]string, 0, len(os.Environ())+len(cfg.Env))
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}
	for _, e := range os.Environ() {
		env = append(env, e)
	}

	s.configs[cfg.Name] = processConfig{
		name:         cfg.Name,
		command:      cfg.Command,
		args:         cfg.Args,
		cwd:          cfg.Cwd,
		env:          env,
		autoRestart:  cfg.AutoRestart,
		maxRestarts:  cfg.MaxRestarts,
		restartDelay: cfg.RestartDelay,
		group:        cfg.Group,
	}

	s.processes[cfg.Name] = &process{
		cfg:    s.configs[cfg.Name],
		status: domain.StatusStopped,
	}

	return nil
}

func (s *supervisor) Deregister(ctx context.Context, name string) error {
	s.mu.Lock()
	_, exists := s.configs[name]
	if !exists {
		s.mu.Unlock()
		return ErrProcessNotFound
	}
	p, hasProcess := s.processes[name]
	s.mu.Unlock()

	if hasProcess && p != nil {
		s.Stop(ctx, name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.configs, name)
	delete(s.processes, name)
	delete(s.ptys, name)
	return nil
}

func (s *supervisor) Start(ctx context.Context, name string) error {
	s.mu.Lock()
	p, exists := s.processes[name]
	if !exists {
		s.mu.Unlock()
		return ErrProcessNotFound
	}
	if p.status == domain.StatusRunning || p.status == domain.StatusStarting {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}

	cfg := p.cfg
	p.status = domain.StatusStarting
	s.mu.Unlock()

	cmd := exec.Command(cfg.command, cfg.args...)
	cmd.Dir = cfg.cwd
	cmd.Env = cfg.env
	configureSysProcAttr(cmd)

	ptmx, err := startPTY(cmd)
	if err != nil {
		s.mu.Lock()
		p.status = domain.StatusStopped
		s.mu.Unlock()
		return fmt.Errorf("supervisor: start pty: %w", err)
	}

	if err := cmd.Start(); err != nil {
		ptmx.Close()
		s.mu.Lock()
		p.status = domain.StatusStopped
		s.mu.Unlock()
		return fmt.Errorf("supervisor: start cmd: %w", err)
	}

	s.mu.Lock()
	p.pid = cmd.Process.Pid
	now := time.Now()
	p.startedAt = &now
	p.status = domain.StatusRunning
	s.ptys[name] = ptmx
	s.mu.Unlock()

	s.wg.Add(1)
	go s.readLoop(name, ptmx, cmd)

	s.wg.Add(1)
	go s.sampleLoop(name, p)

	s.bus.Publish(events.NewEvent(events.KindProcessStarted,
		events.ProcessEvent{State: p.snapshot()}))

	return nil
}

func (s *supervisor) readLoop(name string, r io.Reader, cmd *exec.Cmd) {
	defer s.wg.Done()

	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			line := domain.LogLine{
				Process:   name,
				Stream:    domain.Stdout,
				Timestamp: time.Now(),
				Raw:       buf[:n],
				Seq:       time.Now().UnixNano(),
			}
			if s.logStore != nil {
				s.logStore.Append(line)
			}

			if s.detector != nil {
				match := s.detector.Check(name, line)
				if match != nil {
					s.bus.Publish(events.NewEvent(events.KindAttentionNeeded,
						events.AttentionEvent{Event: domain.AttentionEvent{
							Process:   name,
							Pattern:   match.Pattern,
							Context:   match.Context,
							Timestamp: time.Now(),
						}}))
				}
			}

			s.bus.Publish(events.NewEvent(events.KindLogLine,
				events.LogEvent{Line: line}))
		}
		if err != nil {
			break
		}
	}

	cmd.Wait()

	exitCode := cmd.ProcessState.ExitCode()
	s.mu.Lock()
	p := s.processes[name]
	if p != nil {
		p.mu.Lock()
		if p.status == domain.StatusRunning {
			p.exitCode = &exitCode
			if exitCode != 0 {
				now := time.Now()
				p.crashedAt = &now
				p.status = domain.StatusCrashed
				s.bus.Publish(events.NewEvent(events.KindProcessCrashed,
					events.ProcessEvent{State: p.snapshot()}))
				if p.cfg.autoRestart && p.restartCount < p.cfg.maxRestarts {
					s.wg.Add(1)
					go s.restartWithBackoff(name, p)
				}
			} else {
				p.status = domain.StatusStopped
				s.bus.Publish(events.NewEvent(events.KindProcessStopped,
					events.ProcessEvent{State: p.snapshot()}))
			}
		}
		p.mu.Unlock()
	}
	s.mu.Unlock()
}

func (s *supervisor) restartWithBackoff(name string, p *process) {
	defer s.wg.Done()

	delay := p.cfg.restartDelay * (1 << p.restartCount)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	s.mu.Lock()
	p.status = domain.StatusRestarting
	s.mu.Unlock()

	s.bus.Publish(events.NewEvent(events.KindProcessRestarting,
		events.ProcessEvent{State: p.snapshot()}))

	select {
	case <-time.After(delay):
	case <-s.ctx.Done():
		return
	}

	p.mu.Lock()
	p.restartCount++
	p.mu.Unlock()

	if err := s.Start(context.Background(), name); err != nil {
		p.mu.Lock()
		p.status = domain.StatusCrashed
		p.mu.Unlock()
	}
}

func (s *supervisor) sampleLoop(name string, p *process) {
	defer s.wg.Done()

	ticker := time.NewTicker(resourceSampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			pid := p.pid
			s.mu.RUnlock()

			if pid <= 0 {
				continue
			}

			cpu, mem := readProcessResources(pid)
			p.mu.Lock()
			p.cpuPercent = cpu
			p.memoryMB = mem
			snap := p.snapshot()
			p.mu.Unlock()

			s.bus.Publish(events.NewEvent(events.KindResourceSample,
				events.ProcessEvent{State: snap}))
		}
	}
}

func (s *supervisor) Stop(ctx context.Context, name string) error {
	s.mu.Lock()
	p, exists := s.processes[name]
	if !exists {
		s.mu.Unlock()
		return ErrProcessNotFound
	}
	if p.status == domain.StatusStopped {
		s.mu.Unlock()
		return ErrNotRunning
	}
	ptyFile, _ := s.ptys[name]
	s.mu.Unlock()

	if ptyFile != nil {
		ptyFile.Close()
	}

	s.mu.Lock()
	p.mu.Lock()
	pid := p.pid
	p.mu.Unlock()
	s.mu.Unlock()

	if pid > 0 {
		killProcessGroup(pid)

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- nil
		}()

		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			killProcessGroupForce(pid)
		}

		<-waitCh
	}

	s.mu.Lock()
	p.mu.Lock()
	p.status = domain.StatusStopped
	p.pid = 0
	p.mu.Unlock()
	delete(s.ptys, name)
	s.mu.Unlock()

	s.bus.Publish(events.NewEvent(events.KindProcessStopped,
		events.ProcessEvent{State: p.snapshot()}))

	return nil
}

func (s *supervisor) Restart(ctx context.Context, name string) error {
	if err := s.Stop(ctx, name); err != nil {
		return err
	}
	return s.Start(ctx, name)
}

func (s *supervisor) SendInput(ctx context.Context, name string, input []byte) error {
	s.mu.RLock()
	ptyFile, exists := s.ptys[name]
	s.mu.RUnlock()
	if !exists {
		return ErrProcessNotFound
	}

	_, err := ptyFile.Write(input)
	return err
}

func (s *supervisor) State(name string) (domain.ProcessState, error) {
	s.mu.RLock()
	p, exists := s.processes[name]
	s.mu.RUnlock()
	if !exists {
		return domain.ProcessState{}, ErrProcessNotFound
	}
	return p.snapshot(), nil
}

func (s *supervisor) States() map[string]domain.ProcessState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]domain.ProcessState, len(s.processes))
	for name, p := range s.processes {
		result[name] = p.snapshot()
	}
	return result
}

func (s *supervisor) Close(ctx context.Context) error {
	s.cancel()

	s.mu.Lock()
	for name := range s.processes {
		s.Stop(ctx, name)
	}
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}
