package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddJob(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}
	url := "http://jenkins/job/test"

	c.AddJob(url)
	assert.True(t, c.HasJob(url), "expected job to exist after AddJob")

	job, exists := c.Jobs[url]
	assert.True(t, exists, "job not found in map")
	assert.Equal(t, url, job.URL)
	assert.False(t, job.StartTime.IsZero(), "expected StartTime to be set")
}

func TestAddJob_NoDuplicates(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}
	url := "http://jenkins/job/test"

	c.AddJob(url)
	originalTime := c.Jobs[url].StartTime

	c.AddJob(url)
	assert.Equal(t, originalTime, c.Jobs[url].StartTime, "AddJob should not overwrite existing job")
	assert.Equal(t, 1, len(c.Jobs), "expected 1 job, got %d", len(c.Jobs))
}

func TestUpdateJobCheckStatus(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}
	url := "http://jenkins/job/test"

	c.AddJob(url)

	changed := c.UpdateJobCheckStatus(url, true)
	assert.True(t, changed, "expected UpdateJobCheckStatus to return true when status changes")
	assert.True(t, c.Jobs[url].LastCheckFailed, "expected LastCheckFailed to be true")

	changed = c.UpdateJobCheckStatus(url, true)
	assert.False(t, changed, "expected UpdateJobCheckStatus to return false when status unchanged")

	changed = c.UpdateJobCheckStatus(url, false)
	assert.True(t, changed, "expected UpdateJobCheckStatus to return true when status changes")
	assert.False(t, c.Jobs[url].LastCheckFailed, "expected LastCheckFailed to be false")

	changed = c.UpdateJobCheckStatus(url, false)
	assert.False(t, changed, "expected UpdateJobCheckStatus to return false when status unchanged")
}

func TestUpdateJobCheckStatus_NonExistentJob(t *testing.T) {
	c := &Config{Jobs: make(map[string]Job)}

	changed := c.UpdateJobCheckStatus("http://jenkins/job/nonexistent", true)
	assert.False(t, changed, "expected UpdateJobCheckStatus to return false for non-existent job")

	assert.Equal(t, 0, len(c.Jobs), "should not create job when updating non-existent")
}

func TestUpdate_NoDeadlockWithDirectModification(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	store := NewDiskStore()

	cfg, err := store.Load()
	assert.NoError(t, err, "failed to load config: %v", err)

	url := "http://jenkins/job/test"
	cfg.AddJob(url)
	err = store.Save(cfg)
	assert.NoError(t, err, "failed to save config: %v", err)

	done := make(chan struct{})
	go func() {
		err := store.Update(func(c *Config) error {
			if job, exists := c.Jobs[url]; exists {
				job.LastCheckFailed = true
				c.Jobs[url] = job
			}
			return nil
		})
		assert.NoError(t, err, "Update failed: %v", err)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Update deadlocked - took longer than 2 seconds")
	}

	reloaded, err := store.Load()
	assert.NoError(t, err, "failed to reload config: %v", err)
	assert.True(t, reloaded.Jobs[url].LastCheckFailed, "expected LastCheckFailed to be true after Update")
}

func TestUpdate_RemoveJobDirectDeletion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	store := NewDiskStore()

	cfg, err := store.Load()
	assert.NoError(t, err, "failed to load config: %v", err)

	url := "http://jenkins/job/test"
	cfg.AddJob(url)
	err = store.Save(cfg)
	assert.NoError(t, err, "failed to save config: %v", err)

	done := make(chan struct{})
	go func() {
		err := store.Update(func(c *Config) error {
			delete(c.Jobs, url)
			return nil
		})
		assert.NoError(t, err, "Update failed: %v", err)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Update deadlocked - took longer than 2 seconds")
	}

	reloaded, err := store.Load()
	assert.NoError(t, err, "failed to reload config: %v", err)
	assert.False(t, reloaded.HasJob(url), "expected job to be removed after Update")
}

func TestLoad_ReturnsCachedInstance(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	store := NewDiskStore()

	cfg, err := store.Load()
	assert.NoError(t, err, "failed to load config: %v", err)

	url := "http://jenkins/job/test"
	cfg.AddJob(url)
	err = store.Save(cfg)
	assert.NoError(t, err, "failed to save config: %v", err)

	cfg2, err := store.Load()
	assert.NoError(t, err, "failed to load config: %v", err)
	assert.NotSame(t, cfg, cfg2, "Load() should return a fresh instance")

	diskCfg, err := loadFromDisk()
	assert.NoError(t, err, "failed to load from disk: %v", err)
	delete(diskCfg.Jobs, url)
	err = store.Save(diskCfg)
	assert.NoError(t, err, "failed to save config: %v", err)

	cachedCfg, err := store.Load()
	assert.NoError(t, err, "failed to load config: %v", err)
	assert.False(t, cachedCfg.HasJob(url), "Load() should return a fresh instance reflecting disk")
}
