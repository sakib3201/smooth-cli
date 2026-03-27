# Smooth CLI

<div align="center">

**A terminal dashboard for orchestrating multiple AI agent processes**

[![Go Report Card](https://goreportcard.com/badge/github.com/smoothcli/smooth-cli)](https://goreportcard.com/report/github.com/smoothcli/smooth-cli)
[![Go Reference](https://pkg.go.dev/badge/github.com/smoothcli/smooth-cli.svg)](https://pkg.go.dev/github.com/smoothcli/smooth-cli)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

<img src="docs/demo.gif" alt="Smooth CLI Demo" width="800">

</div>

---

## Overview

Smooth CLI is a **terminal user interface (TUI)** for spawning, supervising, and monitoring multiple AI agent processes. It provides:

- 🖥️ **Real-time TUI Dashboard** — Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss)
- 🔄 **Process Lifecycle Management** — Auto-restart with exponential backoff, PTY integration, and process grouping
- 📊 **REST + WebSocket API** — Control processes programmatically at `http://127.0.0.1:7700`
- 🤖 **MCP Server** — Use as a tool in Claude Desktop / Claude Code via Model Context Protocol
- 🔍 **Attention Detection** — Regex-based pattern matching for prompts, errors, and OSC notifications
- 💾 **Persistent Logs** — Ring buffer + SQLite for cross-session log search
- ⚡ **Hot Reload** — Live config reload without restart
- 🔐 **Permission Gating** — Require consent before applying dangerous config changes

---

## Installation

### Pre-built Binaries

Download from [Releases](https://github.com/smoothcli/smooth-cli/releases).

### Build from Source

```bash
# Clone the repository
git clone https://github.com/smoothcli/smooth-cli.git
cd smooth-cli

# Build
make build

# The binary will be at bin/smooth
./bin/smooth --help
```

### Prerequisites

- **Go 1.23+** — For building from source
- **No CGO required** — Uses pure-Go SQLite driver (`modernc.org/sqlite`)

---

## Quick Start

### 1. Create a Configuration File

Create `smooth.yml` in your project directory:

```yaml
version: 1
project: my-ai-agents

processes:
  api:
    command: "uvicorn main:app --reload"
    cwd: "./api"
    auto_restart: true
    group: backend

  worker:
    command: "python worker.py"
    cwd: "./worker"
    auto_restart: true
    group: backend

  frontend:
    command: "npm run dev"
    cwd: "./frontend"
    group: frontend

settings:
  log_buffer_lines: 10000
  persist_logs: true
  notifications: true
  mcp:
    enabled: true
```

### 2. Run Smooth CLI

```bash
smooth
```

This launches the TUI dashboard. Use keyboard shortcuts to navigate:

| Key | Action |
|-----|--------|
| `↑/k` | Previous process |
| `↓/j` | Next process |
| `r` | Restart selected process |
| `s` | Stop selected process |
| `S` | Start stopped process |
| `/` | Search logs |
| `f` | Toggle follow mode |
| `?` | Show help |
| `q` | Quit |

---

## Configuration Reference

### Process Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | **required** | Command to run |
| `cwd` | string | `.` | Working directory |
| `env` | map | — | Environment variables (supports `${VAR}` interpolation) |
| `auto_restart` | bool | `false` | Restart on crash |
| `max_restarts` | int | `5` | Maximum restart attempts |
| `restart_delay` | duration | `2s` | Base delay for exponential backoff |
| `group` | string | `default` | Process grouping for TUI |
| `attention.patterns` | []string | — | Custom attention patterns (merged with defaults) |

### Global Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `log_buffer_lines` | int | `10000` | Lines kept in memory per process |
| `persist_logs` | bool | `true` | Write logs to SQLite |
| `notifications` | bool | `true` | OS desktop notifications |
| `permission_gating` | bool | `true` | Require consent for dangerous config changes |
| `mcp.enabled` | bool | `true` | Enable MCP server |
| `mcp.dangerous_tools` | bool | `false` | Allow `restart_process` / `run_command` via MCP |

---

## REST API

When running, Smooth CLI exposes a REST API at `http://127.0.0.1:7700`:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/processes` | List all processes |
| `GET` | `/api/v1/processes/:name` | Get process state |
| `POST` | `/api/v1/processes/:name/start` | Start a process |
| `POST` | `/api/v1/processes/:name/stop` | Stop a process |
| `POST` | `/api/v1/processes/:name/restart` | Restart a process |
| `POST` | `/api/v1/processes/:name/input` | Send input to PTY |
| `GET` | `/api/v1/processes/:name/logs` | Get log lines |
| `GET` | `/api/v1/config` | Get current config |
| `POST` | `/api/v1/config/reload` | Trigger hot reload |
| `GET` | `/api/v1/events` | Get event history |
| `GET` | `/ws` | WebSocket event stream |
| `GET` | `/api/v1/health` | Health check |

### Example: Start a Process

```bash
curl -X POST http://127.0.0.1:7700/api/v1/processes/api/start
# {"status":"started"}
```

### Example: Stream Events via WebSocket

```javascript
const ws = new WebSocket('ws://127.0.0.1:7700/ws');
ws.onopen = () => ws.send(JSON.stringify({
  type: 'subscribe',
  kinds: ['process.started', 'log.line', 'attention.needed']
}));
ws.onmessage = (event) => console.log(JSON.parse(event.data));
```

---

## MCP Server

Smooth CLI can run as an MCP (Model Context Protocol) server, enabling AI assistants like Claude to interact with your processes:

```bash
# Run MCP server on stdio
smooth mcp serve
```

### Available Tools

| Tool | Description | Permissions |
|------|-------------|-------------|
| `list_processes` | List all managed processes | — |
| `get_logs` | Get recent log lines for a process | — |
| `get_attention_events` | Get unresolved attention events | — |
| `get_config` | Get current smooth.yml config | — |
| `restart_process` | Restart a managed process | Requires `dangerous_tools: true` |
| `run_command` | Send input text to a process PTY | Requires `dangerous_tools: true` |

### Claude Desktop Configuration

Add to your Claude Desktop config:

```json
{
  "mcpServers": {
    "smooth": {
      "command": "smooth",
      "args": ["mcp", "serve"]
    }
  }
}
```

---

## Attention Detection

Smooth CLI automatically detects when processes need user attention:

**Built-in Patterns:**
- `(y/n)`, `[yes/no]` — User prompts
- `error:`, `FATAL:`, `panic:` — Errors
- `password:`, `Enter your API key:` — Credential prompts
- `permission denied`, `access denied` — Permission issues
- OSC sequences (9, 99, 777) — Terminal notifications

**Custom Patterns:**

```yaml
processes:
  api:
    command: "node server.js"
    attention:
      patterns:
        - "(?i)custom error pattern"
        - "(?i)waiting for input"
```

---

## Development

### Project Structure

```
smooth-cli/
├── cmd/smooth/           # CLI entry point
├── internal/
│   ├── api/              # REST + WebSocket server
│   ├── attention/        # Regex detector + corpus
│   ├── cloud/            # Cloud sync client
│   ├── config/           # Config parser + hot-reload
│   ├── domain/           # Shared value types
│   ├── events/           # Typed pub/sub bus
│   ├── logstore/         # Ring buffer + SQLite persistence
│   ├── mcp/              # MCP server implementation
│   ├── notify/           # OS notification wrapper
│   ├── permission/        # Consent flow for config changes
│   ├── store/            # In-memory state + SQLite
│   ├── supervisor/       # Process lifecycle manager
│   └── tui/              # Bubble Tea UI + panes
├── testdata/             # Test fixtures
├── ai-docs/              # Specs + implementation plan
└── .github/workflows/    # CI/CD
```

### Running Tests

```bash
# Unit tests with race detector
make test-race

# Integration tests
make test-integration

# Coverage report
make test-cover

# Lint
make lint
```

### Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   TUI       │ ←→ │  Event Bus  │ ←→ │ Supervisor  │
│  (Bubble   │     │  (fan-out)  │     │  (PTY)      │
│   Tea)      │     └─────────────┘     └─────────────┘
└─────────────┘            │                    │
      │                    ↓                    ↓
      │            ┌─────────────┐     ┌─────────────┐
      │            │   Store    │     │  LogStore   │
      │            │  (SQLite)   │     │ (RingBuf)   │
      │            └─────────────┘     └─────────────┘
      │
┌─────┴───────┐
│ REST/WS API │
│   (Chi)     │
└─────────────┘
      │
┌─────┴───────┐
│ MCP Server  │
│  (mcp-go)   │
└─────────────┘
```

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test-race`)
5. Commit with conventional commits (`feat: add amazing feature`)
6. Push to your fork
7. Open a Pull Request

---

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) — see the [LICENSE](LICENSE) file for details.

---

## Acknowledgments

Built with ❤️ using:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Terminal styling
- [Chi](https://github.com/go-chi/chi) — HTTP router
- [mcp-go](https://github.com/mark3labs/mcp-go) — MCP implementation
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — Pure Go SQLite

---

<div align="center">

**[Documentation](docs/)** · **[Report Bug](https://github.com/smoothcli/smooth-cli/issues)** · **[Request Feature](https://github.com/smoothcli/smooth-cli/issues)**

Made with ☕ by [Smooth CLI Contributors](https://github.com/smoothcli/smooth-cli/graphs/contributors)

</div>