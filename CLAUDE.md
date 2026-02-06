# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`jw` is a Go CLI daemon that monitors Jenkins jobs in the background and sends macOS notifications when they complete. It uses Cobra for CLI commands and runs as a background daemon process.

## Build & Test Commands

```bash
go build -o jw .                          # Build binary
go test ./...                             # Run all tests
go test -v ./pkg/config                   # Run tests for a specific package
go test -run TestAddJob ./pkg/config      # Run a single test
go install github.com/baggiiiie/jw@latest # Install from registry
```

## Architecture

**CLI layer (`cmd/`):** Cobra-based commands. `main.go` delegates to `cmd.RootCmd`. The daemon is started via a hidden `_start_jw_daemon` subcommand that detaches from the terminal (`setsid`).

**Business logic (`pkg/`):**
- `config` — Job list persistence in `~/.jw/monitored_jobs.json` with file locking (`syscall.Flock`). Uses a `ConfigStore` interface with `DiskStore` implementation (dependency injection, not singleton).
- `jenkins` — HTTP client for Jenkins REST API. Polls `/api/json?tree=building,result,timestamp`. Expects pre-encoded base64 credentials.
- `monitor` — Polling loop (`MonitorJob`) that checks job status every 30s, sends notifications, and updates config. Communicates completion back to daemon via channels.
- `notify` — macOS-only notifications via `terminal-notifier` or `osascript` fallback.
- `pidfile` — PID file management with self-healing (restores missing PID files via `pgrep`).
- `logging`, `color`, `version`, `upgrade` — Supporting utilities.

**Daemon lifecycle (`cmd/daemon.go`):** Signal-driven event loop using `select`. Responds to SIGHUP (reload config), SIGINT/SIGTERM (shutdown). Spawns a goroutine per monitored job with stop channels. Auto-shuts down when no jobs remain.

**Credentials:** Requires either `JENKINS_USER` + `JENKINS_API_TOKEN` or legacy `JENKINS_TOKEN` env vars. Token validation logic is currently duplicated between `cmd/add.go` and `cmd/daemon.go`.

## Key Design Decisions

- `ConfigStore` interface enables dependency injection — the previous singleton was replaced to eliminate deadlock footguns (see `improvements.md` for context).
- `DiskStore.Update()` holds a file lock and reloads from disk before applying mutations, ensuring atomic read-modify-write.
- Tests use `testify` for assertions and `httptest` for Jenkins API mocking.
- The config package tests verify no deadlocks occur when modifying config within `Update()` callbacks.

## Runtime Files

All stored in `~/.jw/`:
- `monitored_jobs.json` — job state
- `.jenkins_monitor.pid` — daemon PID
- `.jenkins_monitor.log` — daemon logs
- `config.lock` — file lock
