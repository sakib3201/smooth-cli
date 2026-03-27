package styles

import "github.com/charmbracelet/lipgloss"

var (
	ColourGreen  = lipgloss.Color("#00C87A")
	ColourAmber  = lipgloss.Color("#F5A623")
	ColourBlue   = lipgloss.Color("#4A9EE0")
	ColourRed    = lipgloss.Color("#E55353")
	ColourGrey   = lipgloss.Color("#8A9BA8")
	ColourCyan   = lipgloss.Color("#26C6DA")
	ColourBorder = lipgloss.Color("#3C4048")
)

var (
	BadgeRunning    = lipgloss.NewStyle().Foreground(ColourGreen).Bold(true)
	BadgeAttention  = lipgloss.NewStyle().Foreground(ColourAmber).Bold(true)
	BadgeRestarting = lipgloss.NewStyle().Foreground(ColourBlue)
	BadgeCrashed    = lipgloss.NewStyle().Foreground(ColourRed).Bold(true)
	BadgeStopped    = lipgloss.NewStyle().Foreground(ColourGrey)
	BadgeStarting   = lipgloss.NewStyle().Foreground(ColourCyan)
)

var (
	PaneBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColourBorder)
	StatusBar  = lipgloss.NewStyle().Background(lipgloss.Color("#1A1B26")).Foreground(ColourGrey).Padding(0, 1)
	Title      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CDD6F4"))
	Selected   = lipgloss.NewStyle().Background(lipgloss.Color("#313244")).Bold(true)
)

func StatusSymbol(status string, tick bool) string {
	switch status {
	case "running":
		return "●"
	case "attention":
		if tick {
			return "◉"
		}
		return "◎"
	case "restarting":
		if tick {
			return "↺"
		}
		return "↻"
	case "crashed":
		return "✕"
	case "stopped":
		return "○"
	case "starting":
		return "⧗"
	default:
		return "?"
	}
}
