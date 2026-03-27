package permissionpane

import (
	"github.com/smoothcli/smooth-cli/internal/domain"
)

type Model struct {
	Requests []domain.PermissionRequest
	Visible  bool
	Index    int
}

func New() Model {
	return Model{}
}

func (m Model) AddRequest(req domain.PermissionRequest) Model {
	m.Requests = append(m.Requests, req)
	m.Visible = true
	return m
}

func (m Model) ApproveCurrent() Model {
	if len(m.Requests) == 0 {
		return m
	}
	m.Requests = m.Requests[1:]
	if len(m.Requests) == 0 {
		m.Visible = false
	}
	m.Index = 0
	return m
}

func (m Model) DenyCurrent() Model {
	return m.ApproveCurrent()
}
