# JW Testing Strategy

## Overview

This document outlines the recommended testing strategy for the JW (Jenkins Watcher) project. The focus is on **high-impact tests** that cover critical functionality.

## Current State

- **No tests exist** in the codebase
- No test infrastructure or CI test pipeline

## Recommended Test Structure

```
/home/user/jw/
├── pkg/
│   ├── config/
│   │   └── config_test.go      # Config persistence & migration
│   ├── jenkins/
│   │   └── jenkins_test.go     # Jenkins API client
│   ├── monitor/
│   │   └── monitor_test.go     # Job monitoring logic
│   └── pidfile/
│       └── pidfile_test.go     # PID file management
└── cmd/
    └── integration_test.go     # (Optional) CLI integration tests
```

---

## Priority 1: Must-Have Tests

### 1. Config Management (`pkg/config/config_test.go`)

**Why:** Configuration is the heart of state persistence. Bugs here cause data loss.

| Test Case | Description |
|-----------|-------------|
| `TestLoadSave` | Save config, reload, verify data integrity |
| `TestLegacyMigration` | Migrate array-based format to map-based |
| `TestAddRemoveJob` | Add and remove jobs, verify persistence |
| `TestConcurrentAccess` | Multiple goroutines read/write config |
| `TestCorruptedFile` | Handle invalid JSON gracefully |
| `TestEmptyFile` | Handle empty/missing config file |

```go
// Example test structure
func TestLoadSave(t *testing.T) {
    // Create temp file
    // Save config with jobs
    // Load config
    // Assert jobs match
}

func TestLegacyMigration(t *testing.T) {
    // Write legacy array format to temp file
    // Load config (triggers migration)
    // Assert converted to map format
    // Assert original data preserved
}
```

---

### 2. Jenkins API Client (`pkg/jenkins/jenkins_test.go`)

**Why:** External API integration is error-prone. Mock HTTP for reliable tests.

| Test Case | Description |
|-----------|-------------|
| `TestGetJobStatus_Success` | Parse valid building job response |
| `TestGetJobStatus_Finished` | Parse completed job (SUCCESS/FAILURE) |
| `TestGetJobStatus_NotFound` | Handle 404 response |
| `TestGetJobStatus_AuthError` | Handle 401/403 responses |
| `TestGetJobStatus_Timeout` | Handle slow/hanging requests |
| `TestGetJobStatus_InvalidJSON` | Handle malformed responses |

```go
// Example: Use httptest for mocking
func TestGetJobStatus_Success(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify auth header
        assert.Contains(t, r.Header.Get("Authorization"), "Basic")

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]any{
            "building": true,
            "result":   nil,
        })
    }))
    defer server.Close()

    status, code, err := GetJobStatus(server.URL, "token")
    assert.NoError(t, err)
    assert.Equal(t, 200, code)
    assert.True(t, status.Building)
}
```

---

### 3. PID File Management (`pkg/pidfile/pidfile_test.go`)

**Why:** Daemon lifecycle depends on reliable PID tracking. Stale PIDs cause chaos.

| Test Case | Description |
|-----------|-------------|
| `TestWriteRead` | Write PID, read back, verify |
| `TestIsDaemonRunning_Running` | Detect actually running process |
| `TestIsDaemonRunning_Stale` | Detect and clean stale PID file |
| `TestIsDaemonRunning_Missing` | Handle missing PID file |
| `TestCheckAndRestore` | Self-healing PID file restoration |

```go
func TestIsDaemonRunning_Stale(t *testing.T) {
    // Write PID of non-existent process
    tmpFile := writeTempPID(t, 99999)

    running, _ := IsDaemonRunning(tmpFile)
    assert.False(t, running)

    // PID file should be cleaned up
    _, err := os.Stat(tmpFile)
    assert.True(t, os.IsNotExist(err))
}
```

---

### 4. Job Monitor Logic (`pkg/monitor/monitor_test.go`)

**Why:** Core polling logic - bugs here cause missed notifications or infinite loops.

| Test Case | Description |
|-----------|-------------|
| `TestMonitorJob_Completes` | Job finishes, monitoring stops |
| `TestMonitorJob_StopChannel` | External stop signal works |
| `TestMonitorJob_404` | Job deleted mid-monitoring |
| `TestMonitorJob_TransientError` | Continues on temporary failures |

```go
func TestMonitorJob_Completes(t *testing.T) {
    // Mock Jenkins server that returns SUCCESS after 2 polls
    callCount := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        callCount++
        if callCount < 2 {
            json.NewEncoder(w).Encode(map[string]any{"building": true})
        } else {
            json.NewEncoder(w).Encode(map[string]any{"building": false, "result": "SUCCESS"})
        }
    }))

    stopChan := make(chan struct{})
    done := MonitorJob(server.URL, "token", stopChan)

    <-done // Wait for completion
    assert.Equal(t, 2, callCount)
}
```

---

## Priority 2: Nice-to-Have Tests

### 5. CLI Integration Tests

Test the CLI commands end-to-end using `exec.Command`:

```go
func TestAddCommand(t *testing.T) {
    // Set up test environment
    os.Setenv("JENKINS_TOKEN", "test-token")

    cmd := exec.Command("go", "run", ".", "add", "http://jenkins/job/test")
    output, err := cmd.CombinedOutput()

    // Assert output and config changes
}
```

### 6. Notification Tests (Mock-based)

```go
func TestSendNotification(t *testing.T) {
    // Mock terminal-notifier execution
    // Verify correct arguments passed
}
```

---

## Implementation Guide

### Step 1: Set Up Test Infrastructure

```bash
# Create test files
touch pkg/config/config_test.go
touch pkg/jenkins/jenkins_test.go
touch pkg/pidfile/pidfile_test.go
touch pkg/monitor/monitor_test.go
```

### Step 2: Add Test Dependencies

```bash
# Optional: Add testify for assertions
go get github.com/stretchr/testify/assert
```

### Step 3: Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./pkg/config/...
```

### Step 4: Add to CI (`.github/workflows/test.yml`)

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -race -cover ./...
```

---

## Summary: Minimum Viable Test Suite

| Package | Tests | Effort | Impact |
|---------|-------|--------|--------|
| `pkg/config` | 6 tests | Medium | **High** - Data integrity |
| `pkg/jenkins` | 6 tests | Low | **High** - API reliability |
| `pkg/pidfile` | 5 tests | Low | **High** - Daemon stability |
| `pkg/monitor` | 4 tests | Medium | **High** - Core functionality |
| **Total** | **21 tests** | ~4 hours | Covers 80% of risk |

Start with `pkg/config` and `pkg/jenkins` tests as they are the most straightforward to implement and provide immediate value.
