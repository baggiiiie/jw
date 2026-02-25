# CLAUDE.md

## Project Overview

`jw` is a Go CLI daemon that monitors Jenkins jobs in the background and sends macOS notifications when they complete. It uses Cobra for CLI commands and runs as a background daemon process.

## Build & Test Commands

```bash
go build -o bin/jw .                      # Build binary
go test ./...                             # Run all tests
go test -v ./pkg/config                   # Run tests for a specific package
go test -run TestAddJob ./pkg/config      # Run a single test
```

## Architecture

**CLI layer (`cmd/`):** Cobra-based commands. `main.go` delegates to `cmd.RootCmd`. The daemon is started via a hidden `_start_jw_daemon` subcommand that detaches from the terminal (`setsid`).

**Business logic (`pkg/`):**
- `config` — Job list persistence in `~/.jw/monitored_jobs.json` with file locking (`syscall.Flock`). Uses a `ConfigStore` interface with `DiskStore` implementation (dependency injection, not singleton).
- `jenkins` — HTTP client for Jenkins REST API. Polls `/api/json?tree=building,result,timestamp`. Expects pre-encoded base64 credentials.
- `monitor` — Polling loop (`MonitorJob`) that checks job status every 30s, sends notifications, and updates config. Communicates completion back to daemon via channels.
- `notify` — macOS-only notifications via `terminal-notifier` or `osascript` fallback.
- `pidfile` — PID file management with self-healing (restores missing PID files via `pgrep`).
- `logging`, `ui`, `version`, `upgrade` — Supporting utilities.

**Daemon lifecycle (`cmd/daemon.go`):** Signal-driven event loop using `select`. Responds to SIGHUP (reload config), SIGINT/SIGTERM (shutdown). Spawns a goroutine per monitored job with stop channels. Auto-shuts down when no jobs remain.

