// Package placeholder exists solely to anchor all module dependencies
// so go mod tidy retains them before any real source files are written.
// This file will be deleted once other packages exist.
package placeholder

import (
	_ "github.com/BurntSushi/toml"
	_ "github.com/charmbracelet/bubbles/list"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/lipgloss"
	_ "github.com/creack/pty"
	_ "github.com/fsnotify/fsnotify"
	_ "github.com/gen2brain/beeep"
	_ "github.com/go-chi/chi/v5"
	_ "github.com/google/uuid"
	_ "github.com/gorilla/websocket"
	_ "github.com/mark3labs/mcp-go/mcp"
	_ "github.com/spf13/cobra"
	_ "github.com/stretchr/testify/assert"
	_ "go.uber.org/goleak"
	_ "golang.org/x/sys/unix"
	_ "gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)
