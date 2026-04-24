//go:build !windows

package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixPty struct {
	*os.File
	cmd *exec.Cmd
}

func startPty(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	return &unixPty{File: ptmx, cmd: cmd}, nil
}

func (u *unixPty) Close() error {
	err := u.File.Close()
	u.cmd.Process.Kill()
	return err
}
