//go:build !windows

package supervisor

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/creack/pty"
)

// startPTY spawns cmd in a new PTY and returns an io.ReadWriteCloser for the
// PTY master. The process is started inside pty.Start.
func startPTY(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	return ptmx, nil
}
