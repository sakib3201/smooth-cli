package logviewer

import (
	"github.com/smoothcli/smooth-cli/internal/domain"
)

type Model struct {
	Lines      []domain.LogLine
	Follow     bool
	SearchText string
	Offset     int
	Width      int
	Height     int
}

func New() Model {
	return Model{
		Follow: true,
	}
}

func (m Model) Append(line domain.LogLine) Model {
	m.Lines = append(m.Lines, line)
	if m.Follow && len(m.Lines) > m.Height {
		m.Offset = len(m.Lines) - m.Height
	}
	return m
}

func (m Model) ToggleFollow() Model {
	m.Follow = !m.Follow
	return m
}
