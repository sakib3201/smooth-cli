package tui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestView_RendersImmediatelyWithDefaultSize(t *testing.T) {
	m := New(Deps{})
	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Processes")
}

func TestView_RendersAfterWindowSizeMsg(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	view := m.View()
	assert.NotEmpty(t, view)
}

func TestView_ShowsProcessList(t *testing.T) {
	m := New(Deps{})
	states := map[string]domain.ProcessState{
		"clock": {Name: "clock", Status: domain.StatusRunning},
		"ping":  {Name: "ping", Status: domain.StatusRunning},
	}
	m = m.InitProcesses(states)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	view := m.View()
	assert.Contains(t, view, "clock")
	assert.Contains(t, view, "ping")
	assert.Contains(t, view, "Processes")
}

func TestView_ShowsShellPaneAfterShellReady(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(ShellReadyMsg{Shell: &noopRWC{}})
	m = updated.(Model)

	view := m.View()
	assert.Contains(t, view, "Shell")
}

func TestView_ShowsLogViewerBeforeShellReady(t *testing.T) {
	m := New(Deps{})
	states := map[string]domain.ProcessState{
		"clock": {Name: "clock", Status: domain.StatusRunning},
	}
	m = m.InitProcesses(states)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	view := m.View()
	assert.Contains(t, view, "Logs")
}

func TestView_ShellDataAccumulatesInBuffer(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(ShellReadyMsg{Shell: &noopRWC{}})
	m = updated.(Model)

	updated, _ = m.Update(ShellDataMsg{Data: []byte("Microsoft Windows [Version 10.0]\r\n")})
	m = updated.(Model)

	view := m.View()
	assert.Contains(t, view, "Microsoft Windows")
}

func TestView_StatusBarContainsKeyHints(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	view := m.View()
	assert.Contains(t, view, "navigate")
	assert.Contains(t, view, "quit")
}

func TestView_TabSwitchesFocus(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(ShellReadyMsg{Shell: &noopRWC{}})
	m = updated.(Model)

	assert.Equal(t, FocusProcessList, m.focus)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab, Runes: []rune{'\t'}})
	m = updated.(Model)
	assert.Equal(t, FocusLogViewer, m.focus)

	view := m.View()
	assert.Contains(t, view, "esc: back to list")
}

func TestView_EscReturnsToProcessList(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(ShellReadyMsg{Shell: &noopRWC{}})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab, Runes: []rune{'\t'}})
	m = updated.(Model)
	assert.Equal(t, FocusLogViewer, m.focus)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, FocusProcessList, m.focus)
}

func TestView_ShellBufferTruncates(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(ShellReadyMsg{Shell: &noopRWC{}})
	m = updated.(Model)

	big := strings.Repeat("x", 60000)
	updated, _ = m.Update(ShellDataMsg{Data: []byte(big)})
	m = updated.(Model)

	assert.LessOrEqual(t, len(m.shellBuf), 50000)
}

func TestView_ProcessUpdateModifiesState(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(ProcessUpdateMsg(domain.ProcessState{
		Name:   "clock",
		Status: domain.StatusRunning,
	}))
	m = updated.(Model)

	assert.Equal(t, domain.StatusRunning, m.processes["clock"].Status)
	view := m.View()
	assert.Contains(t, view, "clock")
}

func TestView_NavigateBetweenProcesses(t *testing.T) {
	m := New(Deps{})
	states := map[string]domain.ProcessState{
		"clock": {Name: "clock", Status: domain.StatusRunning},
		"ping":  {Name: "ping", Status: domain.StatusRunning},
	}
	m = m.InitProcesses(states)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	assert.Equal(t, 1, m.processList.Selected)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	assert.Equal(t, 0, m.processList.Selected)
}

func TestView_LogLineAppendsWhenProcessSelected(t *testing.T) {
	m := New(Deps{})
	states := map[string]domain.ProcessState{
		"clock": {Name: "clock", Status: domain.StatusRunning},
	}
	m = m.InitProcesses(states)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(LogLineMsg(domain.LogLine{
		Process: "clock",
		Raw:     []byte("hello from clock"),
	}))
	m = updated.(Model)

	assert.Len(t, m.logViewer.Lines, 1)
	assert.Equal(t, "hello from clock", string(m.logViewer.Lines[0].Raw))
}

type noopRWC struct{}

func (n *noopRWC) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (n *noopRWC) Write(b []byte) (int, error) { return len(b), nil }
func (n *noopRWC) Close() error                 { return nil }
