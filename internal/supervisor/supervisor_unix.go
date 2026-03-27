//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

func configureSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

func killProcessGroupForce(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
