# AGENTS.md

## Commands
- Build: `go build -o jw .`
- Test all: `go test ./...`
- Test package: `go test -v ./pkg/config`
- Test single: `go test -run TestAddJob ./pkg/config`
- Typecheck: `go vet ./...`

## Architecture
Go CLI daemon (module `jenkins-monitor`) that monitors Jenkins jobs and sends macOS notifications. Cobra for CLI, `testify` for tests, `httptest` for API mocking.
- `cmd/` — Cobra commands (`root.go` entry). Daemon runs via hidden `_start_jw_daemon` subcommand with signal-driven event loop (`SIGHUP` reload, `SIGINT`/`SIGTERM` shutdown).
- `pkg/config` — Job persistence in `~/.jw/monitored_jobs.json` with file locking. `ConfigStore` interface with `DiskStore` impl (dependency injection). `Update()` holds lock and reloads before mutating.
- `pkg/jenkins` — HTTP client for Jenkins REST API. Expects pre-encoded base64 credentials.
- `pkg/monitor` — Polling loop (30s interval), sends notifications, updates config. Uses channels for completion.
- `pkg/notify` — macOS notifications via `terminal-notifier` or `osascript` fallback.
- `pkg/pidfile`, `pkg/logging`, `pkg/color`, `pkg/version`, `pkg/upgrade` — Supporting utilities.

## Code Style
- Standard Go conventions (`gofmt`). No comments unless complex. Use `fmt.Errorf` with `%w` for error wrapping.
- Prefer `ConfigStore` interface over direct `DiskStore` usage. Tests use `testify/assert` and `testify/require`.
- Env vars for credentials: `JENKINS_USER` + `JENKINS_API_TOKEN` (or legacy `JENKINS_TOKEN`).
