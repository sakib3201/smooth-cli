package tui

import (
	"io"

	"github.com/charmbracelet/bubbletea"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/supervisor"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/attention"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/logviewer"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/permissionpane"
	"github.com/smoothcli/smooth-cli/internal/tui/panes/processlist"
)

type Deps struct {
	Sup supervisor.Supervisor
}

type (
	ProcessUpdateMsg domain.ProcessState
	LogLineMsg       domain.LogLine
	AttentionMsg     domain.AttentionEvent
	TickMsg          struct{ tick bool }
	ShellDataMsg     struct{ Data []byte }
	ShellReadyMsg    struct{ Shell io.ReadWriteCloser }
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
	tick      bool

	sup         supervisor.Supervisor
	shell       io.ReadWriteCloser
	shellBuf    string
	cloudStatus CloudStatus
	keys        KeyMap
}

func New(deps Deps) Model {
	return Model{
		processes: make(map[string]domain.ProcessState),
		focus:     FocusProcessList,
		keys:      DefaultKeyMap,
		sup:       deps.Sup,
		width:     120,
		height:    40,
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) InitProcesses(states map[string]domain.ProcessState) Model {
	m.processes = states
	m.processList = m.processList.UpdateProcesses(states)
	return m
}

func readShell(r io.Reader) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := r.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			return ShellDataMsg{Data: data}
		}
		if err != nil {
			return nil
		}
		return nil
	}
}
