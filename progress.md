# Smooth CLI — Implementation Progress

> Track implementation status across all 26 tasks from the plan in `ai-docs/smooth-cli-implementation-plan.md`.
> Working branch: `feature/implement`
> Worktree: `.worktrees/implement`

---

## How to continue with another tool

1. Read `ai-docs/smooth-cli-implementation-plan.md` for full task details (code, commands, expected output).
2. Read `ai-docs/smooth-cli-tui-spec.md` for interface contracts, types, and test stubs.
3. All work goes in `.worktrees/implement` on branch `feature/implement`.
4. Pick the next `[ ]` task below, implement it following the plan, commit, then mark it `[x]`.

---

## Phase 1 — Project Scaffold

| # | Task | Status | Commit |
|---|---|---|---|
| 1 | Initialise Go module and declare dependencies | ✅ Done | `d105079` |
| 2 | Makefile, linter config, goreleaser, smooth.example.yml | ✅ Done | committed |
| 3 | Test data scripts and fixtures | ✅ Done | committed |

## Phase 2 — Domain Types

| # | Task | Status | Commit |
|---|---|---|---|
| 4 | `internal/domain/types.go` — all shared value types | ✅ Done | committed |

## Phase 3 — Config Loader

| # | Task | Status | Commit |
|---|---|---|---|
| 5 | `internal/config/config.go` — ParseYAML, ParseTOML, Diff, validation | ✅ Done | `efcde06` |
| 6 | `internal/config/watch.go` — fsnotify hot-reload watcher | ⬜ Todo | — |

## Phase 4 — Event Bus

| # | Task | Status | Commit |
|---|---|---|---|
| 7 | `internal/events/bus.go` — typed fan-out pub/sub | ⬜ Todo | — |

## Phase 5 — State Store

| # | Task | Status | Commit |
|---|---|---|---|
| 8 | `internal/store/` — in-memory state + SQLite (modernc.org/sqlite) | ⬜ Todo | — |

## Phase 6 — Notify

| # | Task | Status | Commit |
|---|---|---|---|
| 9 | `internal/notify/` — beeep wrapper + MockNotifier | ⬜ Todo | — |

## Phase 7 — Log Streamer & Ring Buffer

| # | Task | Status | Commit |
|---|---|---|---|
| 10 | `internal/logstore/ringbuf.go` — fixed-capacity FIFO ring buffer | ⬜ Todo | — |
| 11 | `internal/logstore/store.go` — async SQLite log persistence + search | ⬜ Todo | — |

## Phase 8 — Attention Detector

| # | Task | Status | Commit |
|---|---|---|---|
| 12 | `internal/attention/` — regex detector, OSC support, corpus | ⬜ Todo | — |

## Phase 9 — Process Supervisor

| # | Task | Status | Commit |
|---|---|---|---|
| 13 | `internal/supervisor/process.go` — process state machine | ⬜ Todo | — |
| 14 | `internal/supervisor/pty_unix.go`, `pty_windows.go`, `resource.go` — PTY + resource sampler | ⬜ Todo | — |
| 15 | `internal/supervisor/supervisor.go` — lifecycle orchestration, auto-restart, backoff | ⬜ Todo | — |

## Phase 10 — Permission Gate

| # | Task | Status | Commit |
|---|---|---|---|
| 16 | `internal/permission/gate.go` — diff evaluation, consent flow, audit log | ⬜ Todo | — |

## Phase 11 — REST + WebSocket API

| # | Task | Status | Commit |
|---|---|---|---|
| 17 | `internal/api/` — chi router, REST handlers, WS broadcaster | ⬜ Todo | — |

## Phase 12 — MCP Server

| # | Task | Status | Commit |
|---|---|---|---|
| 18 | `internal/mcp/server.go` — 6 tools + 2 resources via mcp-go | ⬜ Todo | — |

## Phase 13 — Cloud Client

| # | Task | Status | Commit |
|---|---|---|---|
| 19 | `internal/cloud/client.go` — token auth, gzip, retry, chmod 600 credentials | ⬜ Todo | — |

## Phase 14 — TUI Shell

| # | Task | Status | Commit |
|---|---|---|---|
| 20 | `internal/tui/styles/` — Lipgloss theme, status badges | ⬜ Todo | — |
| 21 | `internal/tui/panes/processlist/` + `logviewer/` — process list + log viewer panes | ⬜ Todo | — |
| 22 | `internal/tui/panes/attention/` + `permission/` + top-level model, update, view | ⬜ Todo | — |

## Phase 15 — CLI Entry Point

| # | Task | Status | Commit |
|---|---|---|---|
| 23 | `cmd/smooth/root.go`, `version.go`, `mcp_serve.go` — Cobra commands | ⬜ Todo | — |
| 24 | `cmd/smooth/main.go` — full startup wiring | ⬜ Todo | — |

## Phase 16 — Integration Tests

| # | Task | Status | Commit |
|---|---|---|---|
| 25 | `internal/integration/pipeline_test.go` — full pipeline + hot-reload + MCP + WS | ⬜ Todo | — |

## Phase 17 — CI/CD

| # | Task | Status | Commit |
|---|---|---|---|
| 26 | `.github/workflows/ci.yml` — lint, test, build matrix, corpus, goroutine-leak | ⬜ Todo | — |

---

## Key files

| File | Purpose |
|---|---|
| `ai-docs/smooth-cli-tui-spec.md` | Full spec: interfaces, types, test stubs, acceptance criteria |
| `ai-docs/smooth-cli-implementation-plan.md` | Step-by-step implementation plan with code |
| `ai-docs/progress.md` | This file |
| `.worktrees/implement/` | All implementation work happens here |

## Dependencies between tasks

Tasks must be completed in order within each phase. Cross-phase dependencies:
- Tasks 7–26 all depend on Task 4 (domain types) ✅
- Tasks 8, 15 depend on Task 7 (event bus)
- Task 15 depends on Tasks 10, 11, 12 (logstore + attention)
- Tasks 17, 18 depend on Task 15 (supervisor)
- Task 22 depends on Tasks 7, 8 (bus + store)
- Task 24 depends on all prior tasks
- Task 25 depends on Task 24

---

*Last updated: 2026-03-27*
