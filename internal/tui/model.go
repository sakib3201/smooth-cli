package tui

import (
	"github.com/charmbracelet/bubbletea"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/attention"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/logviewer"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/permissionpane"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/processlist"
)

type FocusedPane int

const (
	FocusProcessList FocusedPane = iota
	FocusLogViewer
	FocusAttentionPanel
	FocusPermissionDialog
)

type CloudStatus int

const (
	CloudNone CloudStatus = iota
	CloudSyncing
	CloudSyncOK
	CloudSyncFailed
	CloudUpgradeAvailable
)

type Model struct {
	processList processlist.Model
	logViewer   logviewer.Model
	attention   attention.Model
	permission  permissionpane.Model

	processes map[string]domain.ProcessState
	focus     FocusedPane
	width     int
	height    int
	ready     bool

	cloudStatus CloudStatus
	keys        KeyMap
}

func New() Model {
	return Model{
		processes: make(map[string]domain.ProcessState),
		focus:     FocusProcessList,
		keys:      DefaultKeyMap,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) InitProcesses(states map[string]domain.ProcessState) Model {
	m.processes = states
	m.processList = m.processList.UpdateProcesses(states)
	return m
}
