package jenkins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const httpTimeout = 30 * time.Second

type ContentTypeError struct {
	ContentType string
}

func (e *ContentTypeError) Error() string {
	return fmt.Sprintf("expected JSON response but got %q (server may require authentication or URL is not a Jenkins job)", e.ContentType)
}

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

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		return nil, resp.StatusCode, &ContentTypeError{ContentType: ct}
	}

	var status JobStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, resp.StatusCode, err
	}
	return &status, resp.StatusCode, nil
}
