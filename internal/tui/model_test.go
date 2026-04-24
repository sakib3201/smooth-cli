package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestModel_Init_HasCorrectInitialState(t *testing.T) {
	m := New(Deps{})
	assert.Equal(t, FocusProcessList, m.focus)
	assert.False(t, m.ready)
	assert.NotNil(t, m.processes)
}

func TestModel_Ready_AfterReadyMsg(t *testing.T) {
	m := New(Deps{})
	m.ready = true
	assert.True(t, m.ready)
}

func TestModel_WindowSizeMsg_UpdatesWidthAndHeight(t *testing.T) {
	m := New(Deps{})
	m.width = 120
	m.height = 40
	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

func TestModel_InitProcesses_SetsProcessList(t *testing.T) {
	m := New(Deps{})
	states := map[string]domain.ProcessState{
		"test": {Name: "test", Status: domain.StatusRunning},
	}
	m = m.InitProcesses(states)
	assert.Equal(t, 1, len(m.processes))
}
