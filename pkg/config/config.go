package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Jobs []string `json:"jobs"`
}

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".jenkins_monitor_jobs.json"), nil
}

func Load() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Jobs: []string{}}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
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

	return os.WriteFile(path, data, 0644)
}

func (c *Config) AddJob(job string) {
	for _, j := range c.Jobs {
		if j == job {
			return
		}
	}
	c.Jobs = append(c.Jobs, job)
}

func (c *Config) RemoveJob(job string) {
	var newJobs []string
	for _, j := range c.Jobs {
		if j != job {
			newJobs = append(newJobs, j)
		}
	}
	c.Jobs = newJobs
}

func (c *Config) HasJob(job string) bool {
	for _, j := range c.Jobs {
		if j == job {
			return true
		}
	}
	return false
}
