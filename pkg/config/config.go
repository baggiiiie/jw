// Package config manages the configuration for monitored jobs
package config

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	configDirName  = ".jw"
	configFileName = "monitored_jobs.json"
	lockFileName   = "config.lock"
)

type Job struct {
	StartTime       time.Time `json:"start_time"`
	URL             string    `json:"url"`
	LastCheckFailed bool      `json:"last_check_failed,omitempty"`
}

type UpgradeCheck struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

type Config struct {
	Jobs         map[string]Job `json:"jobs"`
	UpgradeState UpgradeCheck   `json:"upgrade_check"`
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName), nil
}

func GetConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, configFileName), nil
}

func getLockPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, lockFileName), nil
}

func withFileLock(fn func() error) error {
	lockPath, err := getLockPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return fn()
}

// loadFromDisk reads the config file from disk.
func loadFromDisk() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Jobs: make(map[string]Job)}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.Jobs == nil {
		config.Jobs = make(map[string]Job)
	}
	return &config, nil
}

func saveToDisk(config *Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func (c *Config) AddJob(jobURL string) {
	if _, exists := c.Jobs[jobURL]; exists {
		return
	}
	c.Jobs[jobURL] = Job{
		StartTime: time.Now(),
		URL:       jobURL,
	}
}

func (c *Config) RemoveJob(jobURL string) {
	delete(c.Jobs, jobURL)
}

func (c *Config) HasJob(jobURL string) bool {
	_, exists := c.Jobs[jobURL]
	return exists
}

func (c *Config) GetJobs() map[string]Job {
	// Return a copy to prevent concurrent map access
	jobs := make(map[string]Job, len(c.Jobs))
	maps.Copy(jobs, c.Jobs)
	return jobs
}

// UpdateJobCheckStatus updates the check status for a job.
// Returns true if the status changed, false otherwise.
func (c *Config) UpdateJobCheckStatus(jobURL string, failed bool) bool {
	if job, exists := c.Jobs[jobURL]; exists {
		if job.LastCheckFailed == failed {
			return false
		}
		job.LastCheckFailed = failed
		c.Jobs[jobURL] = job
		return true
	}
	return false
}
