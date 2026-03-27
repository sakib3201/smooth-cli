package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/smoothcli/smooth-cli/internal/tui/styles"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	processList := styles.PaneBorder.Width(m.width / 3).Render("Process List")
	logView := styles.PaneBorder.Width(m.width - m.width/3 - 1).Render("Log Viewer")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		processList,
		logView,
	)
}
