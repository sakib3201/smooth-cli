package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSmoke_TUIStartsAndExits(t *testing.T) {
	bin := filepath.Join("..", "..", "bin", "smooth.exe")
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		t.Skip("smooth.exe not built; run go build -o bin/smooth.exe ./cmd/smooth")
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "smooth.yml")
	cfg := []byte(`version: 1
project: smoke-test
processes:
  ping:
    command: ping
    args: ["-n", "1", "127.0.0.1"]
    auto_restart: false
settings:
  log_buffer_lines: 100
  persist_logs: false
`)
	if err := os.WriteFile(cfgPath, cfg, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(bin, "--config", cfgPath, "--port", "0")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	stderrCh := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(stderrPipe)
		stderrCh <- string(data)
	}()

	time.Sleep(3 * time.Second)

	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		cmd.Process.Kill()
	}

	cmd.Wait()

	stderrOut := <-stderrCh
	t.Logf("stderr output:\n%s", stderrOut)

	for _, bad := range []string{"panic", "fatal"} {
		if strings.Contains(stderrOut, bad) {
			t.Errorf("unexpected stderr output containing %q", bad)
		}
	}
}
