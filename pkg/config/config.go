// Package config manages the configuration for monitored jobs
package config

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sync"
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

var (
	instance *Config
	once     sync.Once
	mu       sync.RWMutex
)

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

// Load returns the singleton config instance.
// On first call, it loads from disk. Subsequent calls return the cached instance.
// For short-lived CLI commands, this effectively loads once per invocation.
// For the daemon, all goroutines share the same instance.
func Load() (*Config, error) {
	var loadErr error
	once.Do(func() {
		instance, loadErr = loadFromDisk()
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return instance, nil
}

// Reload forces a reload from disk, replacing the singleton instance.
// Use this when you know the file has been modified externally (e.g., after SIGHUP).
func Reload() (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	var cfg *Config
	err := withFileLock(func() error {
		var loadErr error
		cfg, loadErr = loadFromDisk()
		return loadErr
	})
	if err != nil {
		return nil, err
	}
	instance = cfg
	return instance, nil
}

func (c *Config) Save() error {
	mu.Lock()
	defer mu.Unlock()

	return withFileLock(func() error {
		path, err := GetConfigPath()
		if err != nil {
			return err
		}

		configDir := filepath.Dir(path)
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return err
		}

		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return err
		}

		return os.WriteFile(path, data, 0o644)
	})
}

// Update performs an atomic read-modify-write operation on the config.
// It loads the latest config from disk, applies the modification function,
// and saves it back—all under the file lock to prevent lost updates.
func Update(fn func(*Config) error) error {
	mu.Lock()
	defer mu.Unlock()

	return withFileLock(func() error {
		cfg, err := loadFromDisk()
		if err != nil {
			return err
		}

		if err := fn(cfg); err != nil {
			return err
		}

		path, err := GetConfigPath()
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}

		instance = cfg
		return nil
	})
}

func (c *Config) AddJob(jobURL string) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := c.Jobs[jobURL]; exists {
		return
	}
	c.Jobs[jobURL] = Job{
		StartTime: time.Now(),
		URL:       jobURL,
	}
}

func (c *Config) RemoveJob(jobURL string) {
	mu.Lock()
	defer mu.Unlock()

	delete(c.Jobs, jobURL)
}

func (c *Config) HasJob(jobURL string) bool {
	mu.RLock()
	defer mu.RUnlock()

	_, exists := c.Jobs[jobURL]
	return exists
}

func (c *Config) GetJobs() map[string]Job {
	mu.RLock()
	defer mu.RUnlock()

	// Return a copy to prevent concurrent map access
	jobs := make(map[string]Job, len(c.Jobs))
	maps.Copy(jobs, c.Jobs)
	return jobs
}

// UpdateJobCheckStatus updates the check status for a job.
// Returns true if the status changed, false otherwise.
func (c *Config) UpdateJobCheckStatus(jobURL string, failed bool) bool {
	mu.Lock()
	defer mu.Unlock()

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
