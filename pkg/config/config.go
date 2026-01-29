package config

import (
	"encoding/json"
	"fmt"
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
	mu           sync.Mutex
}

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configFileName), nil
}

func Load() (*Config, error) {
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

	// First, try to unmarshal into the modern map-based struct.
	var config Config
	if err := json.Unmarshal(data, &config); err == nil {
		// If jobs is nil (e.g., empty "{}" config file), initialize it.
		if config.Jobs == nil {
			config.Jobs = make(map[string]Job)
		}
		return &config, nil
	}

	// If the modern unmarshal failed, it might be the legacy array format.
	var legacyConfig struct {
		Jobs []string `json:"jobs"`
	}
	if err := json.Unmarshal(data, &legacyConfig); err != nil {
		// If this also fails, the config is truly corrupt or in an unknown format.
		return nil, fmt.Errorf("failed to unmarshal config as modern or legacy format: %w", err)
	}

	// If we've successfully unmarshaled the legacy format, migrate it.
	config.Jobs = make(map[string]Job)
	for _, jobURL := range legacyConfig.Jobs {
		config.Jobs[jobURL] = Job{
			StartTime: time.Now(),
			URL:       jobURL,
		}
	}

	// Save the newly migrated config.
	if err := config.Save(); err != nil {
		return nil, fmt.Errorf("failed to save migrated config: %w", err)
	}

	return &config, nil
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *Config) UpdateJobCheckStatus(jobURL string, failed bool) {
	if job, exists := c.Jobs[jobURL]; exists {
		job.LastCheckFailed = failed
		c.Jobs[jobURL] = job
	}
}
