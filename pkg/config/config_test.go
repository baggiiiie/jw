package config

import "testing"

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
