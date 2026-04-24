//go:build windows

package supervisor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// pipeRWC wraps separate read (stdout/stderr) and write (stdin) pipes into a
// single io.ReadWriteCloser so the supervisor can use a unified interface.
type pipeRWC struct {
	r *os.File
	w *os.File
}

func (p *pipeRWC) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipeRWC) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p *pipeRWC) Close() error {
	p.r.Close()
	p.w.Close()
	return nil
}

// startPTY starts cmd with stdout/stderr piped out and stdin piped in, then
// returns the combined io.ReadWriteCloser. The process is started here; the
// caller must NOT call cmd.Start() again.
func startPTY(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pty out pipe: %w", err)
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return nil, fmt.Errorf("pty in pipe: %w", err)
	}

	cmd.Stdout = outW
	cmd.Stderr = outW
	cmd.Stdin = inR

	if err := cmd.Start(); err != nil {
		outR.Close()
		outW.Close()
		inR.Close()
		inW.Close()
		return nil, fmt.Errorf("pty start: %w", err)
	}

	// Close the ends the child owns; keep only our ends.
	outW.Close()
	inR.Close()

	return &pipeRWC{r: outR, w: inW}, nil
}
