package config

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const configFileName = ".jenkins_monitor_jobs.json"

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

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configFileName), nil
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

	cfg, err := loadFromDisk()
	if err != nil {
		return nil, err
	}
	instance = cfg
	return instance, nil
}

func (c *Config) Save() error {
	mu.Lock()
	defer mu.Unlock()

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
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

func (c *Config) JobCount() int {
	mu.RLock()
	defer mu.RUnlock()

	return len(c.Jobs)
}

func (c *Config) UpdateJobCheckStatus(jobURL string, failed bool) {
	mu.Lock()
	defer mu.Unlock()

	if job, exists := c.Jobs[jobURL]; exists {
		job.LastCheckFailed = failed
		c.Jobs[jobURL] = job
	}
}
