package cmd

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/jenkins"
	"jenkins-monitor/pkg/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingNotifier struct {
	mu    sync.Mutex
	calls []notifyCall
}

type notifyCall struct {
	Title   string
	Message string
	URL     string
}

func (r *recordingNotifier) Send(title, message, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, notifyCall{Title: title, Message: message, URL: url})
	return nil
}

func (r *recordingNotifier) getCalls() []notifyCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]notifyCall, len(r.calls))
	copy(out, r.calls)
	return out
}

var _ notify.Notifier = (*recordingNotifier)(nil)

func TestAddMonitorRemoveIntegration(t *testing.T) {
	// --- Fake Jenkins server ---
	var finished atomic.Bool
	var requestCount atomic.Int32
	var capturedAuth atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		capturedAuth.Store(r.Header.Get("Authorization"))

		var status jenkins.JobStatus
		if finished.Load() {
			status = jenkins.JobStatus{Building: false, Result: "SUCCESS", Timestamp: time.Now().UnixMilli()}
		} else {
			status = jenkins.JobStatus{Building: true, Result: "", Timestamp: time.Now().UnixMilli()}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	// --- Isolated HOME ---
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".jw"), 0o755))

	// --- Seed config with the fake job ---
	store := config.NewDiskStore()
	jobURL := server.URL + "/job/test-job"
	err := store.Update(func(cfg *config.Config) error {
		cfg.AddJob(jobURL)
		return nil
	})
	require.NoError(t, err)

	cfg, err := store.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Jobs, 1, "job should be seeded in config")

	// --- Deps ---
	token := base64.StdEncoding.EncodeToString([]byte("test:fake"))
	notifier := &recordingNotifier{}
	stopChan := make(chan struct{})
	logger := log.New(os.Stderr, "test-daemon: ", log.LstdFlags)

	deps := DaemonDeps{
		Store:          store,
		Notifier:       notifier,
		Token:          token,
		SigChan:        make(chan os.Signal),
		Stop:           stopChan,
		PollInterval:   50 * time.Millisecond,
		TickerInterval: 50 * time.Millisecond,
		OnTick:         func() {},
	}

	// --- Run daemon loop ---
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- runDaemonLoop(deps, logger)
	}()

	// Wait until the fake server has received at least one request before flipping.
	require.Eventually(t, func() bool {
		return requestCount.Load() >= 1
	}, 5*time.Second, 10*time.Millisecond, "fake server should have received at least one request")

	// Flip the fake server to return SUCCESS.
	finished.Store(true)

	// Wait for the daemon to auto-exit (no more active jobs).
	select {
	case err := <-doneCh:
		assert.NoError(t, err, "daemon loop should exit without error")
	case <-time.After(5 * time.Second):
		t.Fatal("daemon loop did not exit within timeout")
	}

	// --- Assertions ---

	// Config should have zero jobs.
	cfg, err = store.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Jobs, "config should have zero jobs after completion")

	// Notification should have been sent exactly once.
	calls := notifier.getCalls()
	require.Len(t, calls, 1, "expected exactly one notification")
	assert.Equal(t, "Jenkins Job Completed", calls[0].Title)
	assert.Contains(t, calls[0].Message, "test-job")
	assert.Contains(t, calls[0].Message, "SUCCESS")
	assert.Equal(t, jobURL, calls[0].URL)

	// Auth header should have reached the fake server.
	auth, ok := capturedAuth.Load().(string)
	require.True(t, ok, "auth header should have been captured")
	assert.Equal(t, "Basic "+token, auth, "auth header mismatch")
}
