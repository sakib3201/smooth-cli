package attention

import (
	"github.com/smoothcli/smooth-cli/internal/domain"
)

type Model struct {
	Events  []domain.AttentionEvent
	Visible bool
}

func New() Model {
	return Model{}
}

func (m Model) AddEvent(e domain.AttentionEvent) Model {
	m.Events = append(m.Events, e)
	m.Visible = true
	return m
}

func (m Model) Resolve(process string) Model {
	var remaining []domain.AttentionEvent
	for _, e := range m.Events {
		if e.Process != process {
			remaining = append(remaining, e)
		}
	}
	m.Events = remaining
	if len(m.Events) == 0 {
		m.Visible = false
	}
	return m
}
