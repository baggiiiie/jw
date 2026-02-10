// Package jenkins interacts with Jenkins API
package jenkins

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"jenkins-monitor/pkg/config"
)

// ErrNoCredentials is returned when no Jenkins credentials are configured.
var ErrNoCredentials = errors.New("Jenkins credentials not set. Set JENKINS_USER and JENKINS_API_TOKEN, or JENKINS_TOKEN, or run 'jw auth'")

// GetCredentials returns the base64-encoded credentials for Jenkins Basic Auth.
// It supports two modes:
// 1. JENKINS_USER + JENKINS_API_TOKEN: Combined and base64-encoded (like curl -u user:token)
// 2. JENKINS_TOKEN: Used as-is (legacy, expects pre-encoded value)
// 3. ~/.jw/.credentials file: Checks for stored credentials if env vars are missing
func GetCredentials() (string, error) {
	// 1. Check environment variables first (highest priority)
	user := os.Getenv("JENKINS_USER")
	apiToken := os.Getenv("JENKINS_API_TOKEN")

	if user != "" && apiToken != "" {
		credentials := user + ":" + apiToken
		return base64.StdEncoding.EncodeToString([]byte(credentials)), nil
	}

	token := os.Getenv("JENKINS_TOKEN")
	if token != "" {
		return token, nil
	}

	// 2. Check credentials file
	creds, err := config.LoadCredentials()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("loading credentials file: %w", err)
	}
	if creds != nil && creds.Username != "" && creds.Token != "" {
		credentials := creds.Username + ":" + creds.Token
		return base64.StdEncoding.EncodeToString([]byte(credentials)), nil
	}
	if creds != nil && creds.Token != "" {
		return creds.Token, nil
	}

	return "", ErrNoCredentials
}

const httpTimeout = 30 * time.Second

type JobStatus struct {
	Building  bool   `json:"building"`
	Result    string `json:"result"`
	Timestamp int64  `json:"timestamp"`
}

// GetJobStatus fetches the status of a Jenkins job, and returns the JobStatus
// struct, http status code, and error if any.
func GetJobStatus(jenkinsURL, token string) (*JobStatus, int, error) {
	apiURL := jenkinsURL + "/api/json?tree=building,result,timestamp"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", "Basic "+token)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, fmt.Errorf("job not found (404)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("http error: %s", resp.Status)
	}

	var status JobStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, resp.StatusCode, err
	}
	return &status, resp.StatusCode, nil
}
