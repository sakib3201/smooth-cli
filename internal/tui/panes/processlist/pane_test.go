package processlist

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

func TestPane_ArrowDown_MovesSelection(t *testing.T) {
	m := New()
	m.Processes = map[string]domain.ProcessState{
		"a": {Name: "a"},
		"b": {Name: "b"},
	}
	m.Selected = 0

	m = m.SelectNext()
	assert.Equal(t, 1, m.Selected)

	m = m.SelectNext()
	assert.Equal(t, 0, m.Selected)
}

func TestPane_ArrowUp_MovesSelection(t *testing.T) {
	m := New()
	m.Processes = map[string]domain.ProcessState{
		"a": {Name: "a"},
		"b": {Name: "b"},
	}
	m.Selected = 0

	m = m.SelectPrev()
	assert.Equal(t, 1, m.Selected)
}
