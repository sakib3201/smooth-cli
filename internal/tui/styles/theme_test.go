package styles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusSymbol_Running(t *testing.T) {
	assert.Equal(t, "●", StatusSymbol("running", false))
}

func TestStatusSymbol_Stopped(t *testing.T) {
	assert.Equal(t, "○", StatusSymbol("stopped", false))
}

func TestStatusSymbol_Crashed(t *testing.T) {
	assert.Equal(t, "✕", StatusSymbol("crashed", false))
}

func TestStatusSymbol_Starting(t *testing.T) {
	assert.Equal(t, "⧗", StatusSymbol("starting", false))
}
