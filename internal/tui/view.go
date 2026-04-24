package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/tui/styles"
)

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case TickMsg:
		m.tick = !m.tick
		return m, tickCmd()

	case ProcessUpdateMsg:
		state := domain.ProcessState(msg)
		m.processes[state.Name] = state
		m.processList = m.processList.UpdateProcesses(m.processes)

	case LogLineMsg:
		line := domain.LogLine(msg)
		if line.Process == m.selectedName() {
			m.logViewer = m.logViewer.Append(line)
		}

	case AttentionMsg:
		m.attention = m.attention.AddEvent(domain.AttentionEvent(msg))

	case ShellReadyMsg:
		m.shell = msg.Shell
		if m.shell != nil {
			return m, readShell(m.shell)
		}

	case ShellDataMsg:
		if msg.Data != nil {
			m.shellBuf += string(msg.Data)
			const maxBuf = 50000
			if len(m.shellBuf) > maxBuf {
				m.shellBuf = m.shellBuf[len(m.shellBuf)-maxBuf:]
			}
		}
		if m.shell != nil {
			return m, readShell(m.shell)
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.focus == FocusLogViewer && m.shell != nil {
				m.shell.Write([]byte{0x03})
				return m, nil
			}
			return m, tea.Quit
		}

		if msg.String() == "tab" {
			if m.focus == FocusProcessList {
				m.focus = FocusLogViewer
			} else {
				m.focus = FocusProcessList
			}
			return m, nil
		}

		if msg.String() == "esc" {
			m.focus = FocusProcessList
			return m, nil
		}

		if m.focus == FocusLogViewer && m.shell != nil {
			return m, m.sendShellKey(msg)
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j", "down":
			prev := m.processList.Selected
			m.processList = m.processList.SelectNext()
			if m.processList.Selected != prev {
				m.logViewer.Lines = nil
			}
		case "k", "up":
			prev := m.processList.Selected
			m.processList = m.processList.SelectPrev()
			if m.processList.Selected != prev {
				m.logViewer.Lines = nil
			}
		case "f":
			m.logViewer = m.logViewer.ToggleFollow()
		case "S":
			return m, m.startSelected()
		case "s":
			return m, m.stopSelected()
		case "r":
			return m, m.restartSelected()
		}
	}
	return m, nil
}

func (m Model) sendShellKey(msg tea.KeyMsg) tea.Cmd {
	b := keyToBytes(msg)
	if b != nil {
		shell := m.shell
		return func() tea.Msg {
			shell.Write(b)
			return nil
		}
	}
	return nil
}

func keyToBytes(msg tea.KeyMsg) []byte {
	s := msg.String()
	switch s {
	case "enter":
		return []byte{'\r'}
	case "backspace":
		return []byte{0x08}
	case "tab":
		return []byte{'\t'}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	case "home":
		return []byte("\x1b[H")
	case "end":
		return []byte("\x1b[F")
	case "delete":
		return []byte("\x1b[3~")
	case "pgup":
		return []byte("\x1b[5~")
	case "pgdown":
		return []byte("\x1b[6~")
	case "ctrl+a":
		return []byte{0x01}
	case "ctrl+b":
		return []byte{0x02}
	case "ctrl+d":
		return []byte{0x04}
	case "ctrl+e":
		return []byte{0x05}
	case "ctrl+f":
		return []byte{0x06}
	case "ctrl+k":
		return []byte{0x0b}
	case "ctrl+l":
		return []byte{0x0c}
	case "ctrl+n":
		return []byte{0x0e}
	case "ctrl+p":
		return []byte{0x10}
	case "ctrl+r":
		return []byte{0x12}
	case "ctrl+u":
		return []byte{0x15}
	case "ctrl+w":
		return []byte{0x17}
	}
	if len(msg.Runes) > 0 {
		return []byte(string(msg.Runes))
	}
	return nil
}

func (m Model) selectedName() string {
	names := sortedNames(m.processes)
	if m.processList.Selected < len(names) {
		return names[m.processList.Selected]
	}
	return ""
}

func (m Model) startSelected() tea.Cmd {
	name := m.selectedName()
	if name == "" || m.sup == nil {
		return nil
	}
	return func() tea.Msg {
		_ = m.sup.Start(context.Background(), name)
		return nil
	}
}

func (m Model) stopSelected() tea.Cmd {
	name := m.selectedName()
	if name == "" || m.sup == nil {
		return nil
	}
	return func() tea.Msg {
		_ = m.sup.Stop(context.Background(), name)
		return nil
	}
}

func (m Model) restartSelected() tea.Cmd {
	name := m.selectedName()
	if name == "" || m.sup == nil {
		return nil
	}
	return func() tea.Msg {
		_ = m.sup.Restart(context.Background(), name)
		return nil
	}
}

func (m Model) View() string {
	listWidth := m.width / 3
	logWidth := m.width - listWidth - 4

	listBorder := styles.PaneBorder
	logBorder := styles.PaneBorder
	if m.focus == FocusLogViewer {
		logBorder = styles.PaneBorderFocused
	} else {
		listBorder = styles.PaneBorderFocused
	}

	list := listBorder.Width(listWidth).Height(m.height - 4).Render(
		m.renderProcessList(listWidth),
	)

	var rightContent string
	if m.shell != nil {
		rightContent = m.renderShell(logWidth)
	} else {
		rightContent = m.renderLogViewer()
	}

	log := logBorder.Width(logWidth).Height(m.height - 4).Render(rightContent)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, log)

	var statusText string
	if m.focus == FocusLogViewer && m.shell != nil {
		statusText = "esc: back to list · type to interact with shell · ctrl+c: interrupt"
	} else if m.shell != nil {
		statusText = "j/k: navigate · S: start · s: stop · r: restart · tab: focus shell · q: quit"
	} else {
		statusText = "j/k: navigate · S: start · s: stop · r: restart · tab: focus logs · q: quit"
	}
	statusBar := styles.StatusBar.Width(m.width).Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func (m Model) renderProcessList(width int) string {
	header := styles.Title.Render("Processes")

	if len(m.processes) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, header, styles.BadgeStopped.Render("  no processes configured"))
	}

	names := sortedNames(m.processes)
	rows := []string{header}

	for i, name := range names {
		p := m.processes[name]
		sym := statusSymbol(p.Status, m.tick)
		badge := statusStyle(p.Status)
		label := fmt.Sprintf(" %s %-*s", sym, width-4, name)
		row := badge.Render(label)
		if i == m.processList.Selected {
			row = styles.Selected.Render(label)
		}
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderShell(innerWidth int) string {
	header := styles.Title.Render("Shell")

	lines := strings.Split(m.shellBuf, "\n")
	innerHeight := m.height - 7
	if innerHeight < 1 {
		innerHeight = 1
	}

	start := 0
	if len(lines) > innerHeight {
		start = len(lines) - innerHeight
	}
	visible := lines[start:]

	content := strings.Join(visible, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (m Model) renderLogViewer() string {
	selected := m.selectedName()
	header := styles.Title.Render("Logs")
	if selected != "" {
		header = styles.Title.Render("Logs — " + selected)
	}

	if len(m.logViewer.Lines) == 0 {
		hint := "  no output yet"
		if p, ok := m.processes[selected]; ok && p.Status == domain.StatusStopped {
			hint = "  stopped — press S to start"
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, styles.BadgeStopped.Render(hint))
	}

	var lines []string
	for _, l := range m.logViewer.Lines {
		lines = append(lines, string(l.Raw))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(lines, ""))
}

func sortedNames(processes map[string]domain.ProcessState) []string {
	names := make([]string, 0, len(processes))
	for name := range processes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func statusSymbol(s domain.ProcessStatus, tick bool) string {
	return styles.StatusSymbol(string(s), tick)
}

func statusStyle(s domain.ProcessStatus) lipgloss.Style {
	switch s {
	case domain.StatusRunning:
		return styles.BadgeRunning
	case domain.StatusCrashed:
		return styles.BadgeCrashed
	case domain.StatusRestarting:
		return styles.BadgeRestarting
	case domain.StatusStarting:
		return styles.BadgeStarting
	default:
		return styles.BadgeStopped
	}
}
