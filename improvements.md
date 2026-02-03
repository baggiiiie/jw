# Architecture & Design Improvements

## 1. Config Package: Singleton Anti-Pattern

**Problem:** The config package uses a global singleton with `sync.Once` and module-level mutex. This creates:

- Tight coupling between packages and global state
- Methods on `*Config` that secretly operate on global mutex (`mu`)
- Confusing semantics: `config.Update()` is a function, but `cfg.Save()` is a method that uses the same global lock

**Improvement:** Use dependency injection. Create a `ConfigStore` interface:

```go
type ConfigStore interface {
    Load() (*Config, error)
    Save(*Config) error
    Update(func(*Config) error) error
}
```

Pass `ConfigStore` to daemon/commands rather than using global `config.Load()`.

**Verdict:** ✅ VALID. The global `mu` mutex is used by both package-level `Update()` and method `cfg.Save()`. The `sync.Once` singleton makes testing difficult. The deadlock warning in `monitor.go` (line 110-111) proves this is a real problem—callers must bypass `cfg.UpdateJobCheckStatus()` inside `config.Update()`. DI would eliminate this footgun.

**Implementation notes (2026-02-04):** Replaced the singleton with an injectable `ConfigStore` and a `DiskStore` implementation. CLI commands, daemon, monitor, and upgrade now create a store instance and pass it through, removing `Load/Reload/Update/Save` globals and the shared mutex. Tests now assert that `Load()` returns fresh data from disk instead of cached instances. This removes the deadlock footgun in `monitor.go` and aligns config access with dependency injection.

---

## 2. Mixed Responsibilities in monitor.go

**Problem:** `MonitorJob` does polling, job parsing, notification, AND config updates all in one place. The `updateJobCheckStatusInConfig` function directly manipulates config internals with a comment warning about deadlock.

**Improvement:** Separate concerns:

- `MonitorJob` should only return events via channels
- Let the daemon handle config updates and notifications based on those events
- Define a clean `JobEvent` type: `{URL, Status, Error}`

**Verdict:** ✅ VALID. `MonitorJob` does polling, notification (`notify.Send`), and config updates (`updateJobCheckStatusInConfig`). The function is 100+ lines handling 3 distinct concerns. Separating into event-emitting poller + daemon-side handlers would improve testability and make the deadlock comment unnecessary.

---

## 3. Jenkins Token Handling Scattered

**Problem:** Token validation is duplicated:

- `cmd/add.go` checks `JENKINS_USER`/`JENKINS_API_TOKEN` or `JENKINS_TOKEN`
- `cmd/daemon.go` has `getJenkinsToken()` with the same logic
- `pkg/jenkins/jenkins.go` expects the token already encoded

**Improvement:** Centralize in `pkg/jenkins`:

```go
func GetCredentials() (string, error)  // Returns encoded token or error
```

**Verdict:** ✅ VALID. `cmd/add.go:19-24` and `cmd/daemon.go:34-49` duplicate the same env var checking logic. Moving to `pkg/jenkins` would centralize this and allow `GetJobStatus` to handle its own auth instead of requiring pre-encoded tokens.

---

## 4. Hardcoded Paths and Constants

**Problem:** `.jw` directory name hardcoded in multiple places (`config.go`, `pidfile.go`, `logging.go`). Polling interval (30s), ticker (5s), HTTP timeout (30s) scattered.

**Improvement:**

- Create `pkg/paths` with `GetBaseDir()`, `ConfigPath()`, `LogPath()`, `PidPath()`
- Define constants in one place or make them configurable

**Verdict:** ⚠️ PARTIALLY VALID. `.jw` appears in `config.go:15`, `pidfile.go:17`, `logging.go:14`—confirmed duplication. However, the timeouts/intervals (30s poll, 5s ticker, 30s HTTP) are each used in one place only. A `pkg/paths` would help, but constants consolidation is lower priority.

---

## 5. Daemon Event Loop Complexity

**Problem:** `daemon.go`'s `startDaemon` has a complex loop managing:

- Signals (SIGHUP, SIGINT, SIGTERM)
- Job finish events
- Ticker for PID file checks and auto-shutdown
- Active jobs map

**Improvement:** Extract a `DaemonController` struct:

```go
type DaemonController struct {
    activeJobs map[string]chan struct{}
    events     chan JobEvent
    signals    chan os.Signal
}
func (d *DaemonController) Run(ctx context.Context) error
```

**Verdict:** ⚠️ MINOR ISSUE. `startDaemon` is ~70 lines with a clear select loop. It's not ideal but reasonably readable. The helper functions (`handleJobFinish`, `reloadConfigAndJobs`) already extract logic. A `DaemonController` would help testability but the complexity claim is slightly overstated.

---

## 6. notify.go: macOS-Only, Untestable

**Problem:**

- Hardcoded to macOS (`osascript`, `terminal-notifier`)
- Global `notifierExists` initialized at package load
- Uses `exec.Command` directly, making it hard to test

**Improvement:** Define a `Notifier` interface:

```go
type Notifier interface {
    Send(title, message, url string) error
}
```

Implement `MacOSNotifier`, allow injection for testing.

**Verdict:** ✅ VALID. `notifierExists = checkNotifier()` runs at package init (line 9), calling `exec.LookPath` and potentially `osascript`. This makes the package untestable and non-portable. An interface would allow mocking and cross-platform support.

---

## 7. Error Handling Inconsistency

**Problem:**

- Some functions return errors (`getJenkinsToken` returns `""` instead of error)
- `cfg.Save()` errors sometimes ignored (`_ = cfg.Save()`)
- CLI commands use `os.Exit(1)` directly instead of returning errors

**Improvement:** Consistent error handling:

- Return errors from all fallible functions
- Let cobra's `RunE` handle errors at the command level
- Log or surface ignored errors explicitly

**Verdict:** ✅ VALID. `getJenkinsToken()` returns `("", nil)` on missing creds instead of an error (line 48). `monitor.go:69` ignores `notify.Send` errors with `_ =`. Commands use `os.Exit(1)` directly (add.go:24,29,36) instead of `RunE`. Inconsistent but not critical—worth cleaning up.

---

## 8. HTTP Client Not Reusable

**Problem:** `jenkins.GetJobStatus` creates a new `http.Client` per request.

**Improvement:** Create a `JenkinsClient` struct:

```go
type Client struct {
    baseURL    string
    token      string
    httpClient *http.Client
}
func (c *Client) GetJobStatus(path string) (*JobStatus, error)
```

**Verdict:** ✅ VALID but LOW PRIORITY. `jenkins.go:31` creates `&http.Client{Timeout: httpTimeout}` per request. Go's default transport pools connections, so the performance impact is minimal. A `Client` struct would be cleaner and enable connection reuse, but this isn't a bottleneck given the 30s polling interval.

---

## 9. Job URL Parsing Duplicated

**Problem:** Job name extraction (`strings.Split(jobURL, "/job/")`) appears in:

- `monitor.go` (MonitorJob)
- `status.go` (display)
- `tui.go` (display)

**Improvement:** Create `pkg/jenkins.ParseJobName(url string) string` or add it to a `Job` type.

**Verdict:** ⚠️ PARTIALLY VALID. `monitor.go:18-19` uses `strings.Split(jobURL, "/job/")`. `status.go:45-46` and `tui.go:71-72` use a different approach: `strings.Split(job.URL, "/")` taking last 3 parts. These are actually different extraction strategies, not exact duplication. A helper would still improve consistency.

---

## 10. Upgrade Check in Wrong Place

**Problem:** `RootCmd.PersistentPostRun` runs upgrade check after every command. This means:

- Version check after `jw stop` or `jw logs`
- Unnecessary network call for simple commands

**Improvement:** Only check on specific commands (e.g., `status`, `add`) or use a separate `jw upgrade` command.

**Verdict:** ⚠️ MINOR ISSUE. `root.go:16-22` runs upgrade check in `PersistentPostRun`, but there's already a 24-hour throttle (line 18). The network call only happens once daily, not on every command. The issue exists but the impact is overstated. A dedicated `jw upgrade --check` would still be cleaner.

---

## Summary Priority

| Priority | Issue                             | Impact                | Verdict                          |
| -------- | --------------------------------- | --------------------- | -------------------------------- |
| High     | Config singleton                  | Testability, coupling | ✅ Confirmed, deadlock-prone     |
| High     | Mixed responsibilities in monitor | Maintainability       | ✅ Confirmed                     |
| Medium   | Token handling duplication        | DRY                   | ✅ Confirmed                     |
| Medium   | Daemon complexity                 | Readability           | ⚠️ Overstated, already factored  |
| Medium   | Notify untestable                 | Testability           | ✅ Confirmed (was missing)       |
| Low      | Hardcoded paths                   | DRY                   | ⚠️ Paths yes, constants no       |
| Low      | HTTP client reuse                 | Performance           | ✅ Valid but minimal impact      |
| Low      | Job URL parsing duplication       | DRY                   | ⚠️ Different strategies, not dup |
| Low      | Upgrade check placement           | UX                    | ⚠️ Already throttled daily       |
| Low      | Error handling                    | Consistency           | ✅ Valid                         |
