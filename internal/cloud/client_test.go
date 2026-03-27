package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_ReturnsClient(t *testing.T) {
	c, err := New("https://api.smooth-cli.io")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}
