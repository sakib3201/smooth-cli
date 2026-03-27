//go:build !windows

package supervisor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func startPTY(cmd *exec.Cmd) (*os.File, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	return ptmx, nil
}
