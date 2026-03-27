package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Watch starts a background goroutine that calls onChange whenever the config
// file at path changes and the new config is valid. If the new config is
// invalid, the error is sent to errCh and onChange is not called.
// Runs until ctx is cancelled.
func Watch(ctx context.Context, path string, current *SmoothConfig, onChange func(*SmoothConfig, DiffResult), errCh chan<- error) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("config watch: %w", err)
	}
	if err := w.Add(filepath.Dir(path)); err != nil {
		w.Close()
		return fmt.Errorf("config watch add: %w", err)
	}
	go func() {
		defer w.Close()
		prevVal := *current
		prev := &prevVal
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != filepath.Clean(path) {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("config reload read: %w", err):
					default:
					}
					continue
				}
				ext := filepath.Ext(path)
				var next *SmoothConfig
				switch ext {
				case ".toml":
					next, err = ParseTOML(data)
				default:
					next, err = ParseYAML(data)
				}
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}
				diff := Diff(prev, next)
				onChange(next, diff)
				prev = next
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				select {
				case errCh <- fmt.Errorf("config watcher: %w", err):
				default:
				}
			}
		}
	}()
	return nil
}
