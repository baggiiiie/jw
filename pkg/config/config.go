package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.Jobs == nil {
		config.Jobs = make(map[string]Job)
	}
	return &config, nil
}

func (c *Config) Save() error {
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
