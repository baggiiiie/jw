# Integration Test Plan: Add → Monitor → Remove Workflow

## Goal

Test the full lifecycle in-process: a job is added to config, the daemon monitors it via a fake Jenkins server, the job completes, and the job is automatically removed from config.

> **Review:** Good scope — this covers the highest-value integration path. Consider also asserting that the fake Jenkins server receives the correct `Authorization: Basic <token>` header. It's cheap and catches auth regressions.

## Prerequisites (Refactors)

These changes are required before the test is writable. Each is small and doesn't change production behavior.

### 1. Extract the daemon event loop

`startDaemon()` in `cmd/daemon.go` currently handles PID files, logger setup, signal registration, AND the core `select` loop. Extract the loop into a testable function:

```go
type DaemonDeps struct {
    Store          config.ConfigStore
    Notifier       notify.Notifier
    Token          string
    SigChan        <-chan os.Signal
    Stop           <-chan struct{}
    PollInterval   time.Duration  // passed through to MonitorJob (see #3)
    TickerInterval time.Duration  // auto-shutdown check interval (currently 5s)
}

func runDaemonLoop(deps DaemonDeps, logger *log.Logger) error
```

`startDaemon()` becomes a thin wrapper that sets up logging/PID/signals and calls `runDaemonLoop`.

`runDaemonLoop` should **not** import `pidfile` — PID checks stay in `startDaemon`. The extracted loop only handles the `select` over signals, events, and the auto-shutdown ticker. This keeps the testable core free of OS-level side effects.

> **Review: Production regression risk.** The current ticker branch calls `pidfile.CheckAndRestore()` *inside* the select loop (daemon.go L201). If the loop moves to `runDaemonLoop` but pidfile stays in `startDaemon`, that call is silently lost. Fix: add an `OnTick func()` hook to `DaemonDeps`. Production sets `OnTick = func(){ _ = pidfile.CheckAndRestore() }`, tests set `OnTick = func(){}`.
>
> **Missing `Stop` handling.** Current daemon only exits via signals. `runDaemonLoop` should treat `<-deps.Stop` like SIGTERM (close all active job stop channels, return). Without this, test goroutines leak.
>
> **Events channel.** Current code uses `make(chan monitor.JobEvent, 10)`. Keep this buffered inside `runDaemonLoop` — if it becomes unbuffered, `MonitorJob` goroutines can deadlock while the loop is busy handling an event.
>
> **Initial config load.** `startDaemon` currently `Fatalf`s on load error before entering the loop. Preserve this by having `runDaemonLoop` return an error on initial load failure so the wrapper can still `Fatalf` and tests can assert it.

### 2. Introduce a `Notifier` interface

`handleJobEvent()` in `cmd/daemon.go` calls `notify.Send()` directly, which shells out to `terminal-notifier` or `osascript`. Replace with an interface:

```go
// pkg/notify/notify.go
type Notifier interface {
    Send(title, message, url string) error
}

type MacNotifier struct{}  // existing implementation

func (m MacNotifier) Send(title, message, url string) error { ... }
```

Inject `Notifier` into `DaemonDeps`. `handleJobEvent` uses `deps.Notifier.Send()` instead of calling the package-level `notify.Send()`. Tests inject a recording implementation.

Note: `MonitorJob` does NOT call `notify.Send()` — it only emits `JobEvent`s on a channel. No changes to `MonitorJob` for this step.

> **Review: Import-time side effect.** `pkg/notify/notify.go` has `var notifierExists = checkNotifier()` which runs `exec.LookPath("terminal-notifier")` and may execute `osascript` on import. Even if tests don't use `MacNotifier`, importing the package triggers this. Fix: remove the package-level `var` and make the check lazy inside `MacNotifier.Send()` (cache with `sync.Once`). Do this *before* introducing the interface, otherwise tests/CI can still trigger macOS notification prompts.
>
> **Interface location is fine.** Placing `Notifier` in `pkg/notify` alongside `MacNotifier` is the natural home. Just ensure `EventNotFound` in `handleJobEvent` (daemon.go L71) also uses the injected notifier — the plan only mentions `EventFinished`, but the current code also calls `notify.Send()` for 404s.

### 3. Make poll interval configurable

`MonitorJob` hardcodes `time.NewTicker(30 * time.Second)`. Add a `pollInterval time.Duration` parameter. Tests use a short interval (e.g. 50ms).

Updated signature (only `pollInterval` is added):

```go
func MonitorJob(
    jobURL, token string,
    logger *log.Logger,
    events chan<- JobEvent,
    pollInterval time.Duration,
    stop <-chan struct{},
)
```

Single call-site change in `reloadConfigAndJobs` to pass `deps.PollInterval`.

> **Review: Add a guardrail.** If `pollInterval <= 0`, fall back to the existing 30s default to prevent accidental hot loops in production:
> ```go
> if pollInterval <= 0 { pollInterval = pollingInterval }
> ```
> Otherwise the change is clean — single call-site, no signature surprises. `MonitorJob` already does an immediate first check, so the test doesn't need to wait for the first tick.

## Test Design

File: `cmd/integration_test.go` (or a new `test/` top-level package)

### Setup

1. **Fake Jenkins server** — `httptest.NewServer` with a handler that:
   - Starts returning `{"building": true, "result": "", "timestamp": <now_ms>}`
   - After an `atomic.Bool` is flipped, returns `{"building": false, "result": "SUCCESS", "timestamp": <now_ms>}`

2. **Isolated HOME** — `t.TempDir()` + `t.Setenv("HOME", tmpDir)` + create `tmpDir/.jw/` directory. `DiskStore` picks this up via `os.UserHomeDir()` which reads `$HOME`.

3. **Credentials** — `runDaemonLoop` takes a pre-computed token in `DaemonDeps`, so just pass `base64("test:fake")` directly. No env vars needed.

4. **Recording notifier** — Satisfies the `Notifier` interface, records calls in a mutex-protected slice for assertions.

> **Review: Fake server URL shape matters.** `MonitorJob` extracts the job name via `strings.Split(jobURL, "/job/")`, and `GetJobStatus` appends `/api/json?tree=...`. So the test URL must be `server.URL + "/job/test-job"` and the handler must match `/job/test-job/api/json`. Ensure the handler doesn't break on the `?tree=...` query string.
>
> **HOME isolation is sound.** `os.UserHomeDir()` reads `$HOME` on macOS/Linux, and `t.Setenv` is properly scoped to the test. Just ensure it's set before any config code runs (no package-level init in `pkg/config` that calls `UserHomeDir`, so this is fine).

### Steps

```
1. Create DiskStore pointed at the isolated HOME
2. Add the fake Jenkins URL to config via store.Update()
3. Start runDaemonLoop in a goroutine, capture its return in a done channel:
   - The DiskStore
   - A pre-computed base64 token
   - A no-op signal channel (make(chan os.Signal))
   - A stop channel we control
   - PollInterval = 50ms
   - TickerInterval = 50ms (auto-shutdown check)
   - The recording notifier
4. Wait briefly (~100ms), then flip the fake server's atomic.Bool to return "SUCCESS"
5. Wait for runDaemonLoop to return (it auto-exits when len(activeJobs) == 0)
6. Assert:
   a. Config on disk has zero jobs (store.Load(), len(cfg.Jobs) == 0)
   b. Recording notifier received exactly one Send() call with title "Jenkins Job Completed"
   c. Daemon loop has returned (the done channel is closed)
7. Close stop channel / cleanup
```

> **Review: Biggest flakiness risk is the fixed sleep in step 4.** On a loaded CI runner, 100ms may not be enough for `MonitorJob` to complete its first poll. Replace fixed sleeps with "eventually" waits:
> - Wait until the fake server's request counter (`atomic.Int32`) shows at least one call before flipping the response.
> - Or poll `store.Load()` until `len(cfg.Jobs) == 0` instead of assuming it's done after the daemon returns.
>
> **Auto-exit timing.** Even after `removeJob()` runs, the daemon won't exit until the *next* ticker fires and sees `len(activeJobs) == 0`. With `TickerInterval=50ms` this is fast, but still use a `select` with timeout on the done channel rather than assuming immediate return.
>
> **Cleanup safety.** Use `t.Cleanup` or `defer` with `sync.Once` to close the stop channel — avoids "close of closed channel" panics if the daemon already returned via auto-shutdown.

### Timeout Safety

Wrap the entire test in a `select` with `time.After(5 * time.Second)` to fail fast if something hangs. With 50ms polling and 50ms ticker, the test should complete in under 500ms.

> **Review:** Sound. 5s is generous enough. Ensure the timeout covers all three assertions (daemon exit, notification recorded, config empty) — not just daemon exit.

### What This Validates

- Config is read correctly by the daemon on startup
- `reloadConfigAndJobs` spawns a `MonitorJob` goroutine for the configured job
- Monitor detects the building → finished transition
- `MonitorJob` emits `EventFinished` on the events channel
- `handleJobEvent` dispatches to `Notifier.Send()` and `removeJob()`
- `removeJob` deletes the job from config on disk
- Daemon auto-shuts down when no jobs remain
- Notification is sent with the correct title/message

> **Review:** Good coverage list. One gap: it doesn't mention validating the auth header reaching the Jenkins server. Adding a header check in the fake server would also cover the `jenkins.GetJobStatus` → HTTP request path.

### What This Does NOT Validate

- CLI argument parsing (Cobra layer) — already trivial
- SIGHUP reload (can be a separate test using the same `runDaemonLoop` with a signal channel)
- Real macOS notifications
- PID file management
- Process detachment (`setsid`)

> **Review:** Reasonable exclusions. PID file management could be tested separately with the `OnTick` hook approach — pass a recording `OnTick` and assert it was called.

## Suggested Ordering

1. Refactor: introduce `Notifier` interface in `pkg/notify`
2. Refactor: make poll interval a parameter of `MonitorJob`
3. Refactor: extract `runDaemonLoop` from `startDaemon` (depends on #1 and #2 for the `DaemonDeps` struct)
4. Write the integration test
5. Optionally: add a SIGHUP reload test using the same `runDaemonLoop` harness
6. Optionally: remove `getJenkinsToken()` from `cmd/daemon.go` — it duplicates `jenkins.GetCredentials()` (extracted in `53713ea`)

> **Review: Tweak the ordering.** Step 1 (Notifier interface) should *first* remove the `var notifierExists = checkNotifier()` import-time side effect in `pkg/notify`, otherwise importing the package in tests still triggers `exec.LookPath` / `osascript`. Suggested revised order:
>
> 1. Remove `pkg/notify` import-time side effects (make `checkNotifier` lazy via `sync.Once` in `MacNotifier.Send`)
> 2. Introduce `Notifier` interface + `MacNotifier` impl in `pkg/notify`
> 3. Make poll interval a parameter of `MonitorJob` (with `<=0` fallback guardrail)
> 4. Extract `runDaemonLoop` with `OnTick` hook and `Stop` channel handling
> 5. Write the integration test
> 6. Optional: SIGHUP reload test, remove `getJenkinsToken()` duplicate
