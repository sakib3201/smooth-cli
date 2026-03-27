package supervisor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestProcess_TransitionsToRunning_AfterFirstOutputByte(t *testing.T) {
	p := &process{
		cfg: processConfig{
			name:         "test",
			command:      "echo",
			autoRestart:  false,
			maxRestarts:  5,
			restartDelay: 2 * time.Second,
			group:        "default",
		},
		status: domain.StatusStarting,
	}

	assert.Equal(t, domain.StatusStarting, p.status)
	p.mu.Lock()
	p.status = domain.StatusRunning
	now := time.Now()
	p.startedAt = &now
	p.pid = 1234
	p.mu.Unlock()

	state := p.snapshot()
	assert.Equal(t, domain.StatusRunning, state.Status)
	assert.Equal(t, 1234, state.PID)
}

func TestProcess_TransitionsToRestarting_OnCrash_WhenAutoRestart(t *testing.T) {
	p := &process{
		cfg: processConfig{
			name:         "test",
			command:      "echo",
			autoRestart:  true,
			maxRestarts:  5,
			restartDelay: 2 * time.Second,
			group:        "default",
		},
		status:       domain.StatusRunning,
		restartCount: 2,
	}

	p.mu.Lock()
	p.status = domain.StatusCrashed
	now := time.Now()
	p.crashedAt = &now
	p.mu.Unlock()

	state := p.snapshot()
	assert.Equal(t, domain.StatusCrashed, state.Status)
	assert.Equal(t, 2, state.RestartCount)
}

func TestProcess_DoesNotTransition_FromStopped_Without_Start(t *testing.T) {
	_ = &process{
		cfg: processConfig{
			name:         "test",
			command:      "echo",
			autoRestart:  false,
			maxRestarts:  5,
			restartDelay: 2 * time.Second,
			group:        "default",
		},
		status: domain.StatusStopped,
	}

	assert.False(t, canTransition(domain.StatusStopped, domain.StatusCrashed))
	assert.True(t, canTransition(domain.StatusStopped, domain.StatusStarting))
}

func TestProcess_RejectsInvalidTransitions(t *testing.T) {
	assert.False(t, canTransition(domain.StatusStopped, domain.StatusCrashed))
	assert.False(t, canTransition(domain.StatusStopped, domain.StatusRunning))
	assert.False(t, canTransition(domain.StatusRunning, domain.StatusStarting))
	assert.False(t, canTransition(domain.StatusCrashed, domain.StatusRunning))
}
