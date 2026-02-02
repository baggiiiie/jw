package config

import (
	"sync"
	"testing"
	"time"
)

func TestAddJob(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}
	url := "http://jenkins/job/test"

	c.AddJob(url)
	if !c.HasJob(url) {
		t.Error("expected job to exist after AddJob")
	}

	// Verify job data
	job, exists := c.Jobs[url]
	if !exists {
		t.Fatal("job not found in map")
	}
	if job.URL != url {
		t.Errorf("expected URL %q, got %q", url, job.URL)
	}
	if job.StartTime.IsZero() {
		t.Error("expected StartTime to be set")
	}
}

func TestAddJob_NoDuplicates(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}
	url := "http://jenkins/job/test"

	c.AddJob(url)
	originalTime := c.Jobs[url].StartTime

	c.AddJob(url)
	if c.Jobs[url].StartTime != originalTime {
		t.Error("AddJob should not overwrite existing job")
	}
	if len(c.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(c.Jobs))
	}
}

func TestUpdateJobCheckStatus(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}
	url := "http://jenkins/job/test"

	c.AddJob(url)

	// Update to failed - should return true (changed)
	if changed := c.UpdateJobCheckStatus(url, true); !changed {
		t.Error("expected UpdateJobCheckStatus to return true when status changes")
	}
	if !c.Jobs[url].LastCheckFailed {
		t.Error("expected LastCheckFailed to be true")
	}

	// Update to failed again - should return false (no change)
	if changed := c.UpdateJobCheckStatus(url, true); changed {
		t.Error("expected UpdateJobCheckStatus to return false when status unchanged")
	}

	// Update to success - should return true (changed)
	if changed := c.UpdateJobCheckStatus(url, false); !changed {
		t.Error("expected UpdateJobCheckStatus to return true when status changes")
	}
	if c.Jobs[url].LastCheckFailed {
		t.Error("expected LastCheckFailed to be false")
	}

	// Update to success again - should return false (no change)
	if changed := c.UpdateJobCheckStatus(url, false); changed {
		t.Error("expected UpdateJobCheckStatus to return false when status unchanged")
	}
}

func TestUpdateJobCheckStatus_NonExistentJob(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}

	// Should not panic on non-existent job and return false
	if changed := c.UpdateJobCheckStatus("http://jenkins/job/nonexistent", true); changed {
		t.Error("expected UpdateJobCheckStatus to return false for non-existent job")
	}

	if len(c.Jobs) != 0 {
		t.Error("should not create job when updating non-existent")
	}
}

func TestUpdate_NoDeadlockWithDirectModification(t *testing.T) {
	// This test verifies that modifying config fields directly inside
	// an Update callback doesn't deadlock. This is the correct pattern.
	//
	// Background: config.Update() acquires the global mutex, so callbacks
	// must NOT call mutex-protected methods like UpdateJobCheckStatus().
	// Instead, they should modify cfg.Jobs directly.

	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Reset singleton state for test isolation
	once = sync.Once{}
	instance = nil

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	url := "http://jenkins/job/test"
	cfg.AddJob(url)
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// This should complete without deadlocking
	done := make(chan struct{})
	go func() {
		err := Update(func(c *Config) error {
			// Direct modification - this is the correct pattern
			if job, exists := c.Jobs[url]; exists {
				job.LastCheckFailed = true
				c.Jobs[url] = job
			}
			return nil
		})
		if err != nil {
			t.Errorf("Update failed: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - Update completed without deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("Update deadlocked - took longer than 2 seconds")
	}

	// Verify the modification was applied
	reloaded, err := Reload()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if !reloaded.Jobs[url].LastCheckFailed {
		t.Error("expected LastCheckFailed to be true after Update")
	}
}


func TestUpdate_RemoveJobDirectDeletion(t *testing.T) {
	// This test verifies that removing a job by directly deleting from the map
	// inside an Update callback works without deadlock. This is the correct
	// pattern used by handleJobFinish in daemon.go.

	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Reset singleton state for test isolation
	once = sync.Once{}
	instance = nil

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	url := "http://jenkins/job/test"
	cfg.AddJob(url)
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// This should complete without deadlocking
	done := make(chan struct{})
	go func() {
		err := Update(func(c *Config) error {
			// Direct deletion - this is the correct pattern
			delete(c.Jobs, url)
			return nil
		})
		if err != nil {
			t.Errorf("Update failed: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - Update completed without deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("Update deadlocked - took longer than 2 seconds")
	}

	// Verify the job was removed
	reloaded, err := Reload()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if reloaded.HasJob(url) {
		t.Error("expected job to be removed after Update")
	}
}

