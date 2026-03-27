//go:build windows

package supervisor

import (
	"fmt"
	"os"
	"os/exec"
)

func startPTY(cmd *exec.Cmd) (*os.File, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pty pipe: %w", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, fmt.Errorf("pty start: %w", err)
	}
	pw.Close()
	return pr, nil
}
