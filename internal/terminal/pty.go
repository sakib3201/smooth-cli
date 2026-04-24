package terminal

import (
	"io"
	"os/exec"
)

type PtyProcess struct {
	io.ReadWriteCloser
	Cmd *exec.Cmd
}

func Start(name string, args ...string) (*PtyProcess, error) {
	cmd := exec.Command(name, args...)
	rwc, err := startPty(cmd)
	if err != nil {
		return nil, err
	}
	return &PtyProcess{ReadWriteCloser: rwc, Cmd: cmd}, nil
}

func (p *PtyProcess) Wait() error {
	return p.Cmd.Wait()
}

func (p *PtyProcess) Kill() error {
	return p.Cmd.Process.Kill()
}
