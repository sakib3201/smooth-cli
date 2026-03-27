package processlist

import (
	"github.com/smoothcli/smooth-cli/internal/domain"
)

type Model struct {
	Processes map[string]domain.ProcessState
	Selected  int
	Width     int
	Height    int
}

func New() Model {
	return Model{
		Processes: make(map[string]domain.ProcessState),
	}
}

func (m Model) UpdateProcesses(processes map[string]domain.ProcessState) Model {
	m.Processes = processes
	return m
}

func (m Model) SelectNext() Model {
	if len(m.Processes) == 0 {
		return m
	}
	m.Selected = (m.Selected + 1) % len(m.Processes)
	return m
}

func (m Model) SelectPrev() Model {
	if len(m.Processes) == 0 {
		return m
	}
	if m.Selected == 0 {
		m.Selected = len(m.Processes) - 1
	} else {
		m.Selected--
	}
	return m
}
