package jenkins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type JobStatus struct {
	Building bool   `json:"building"`
	Result   string `json:"result"`
}

func GetJobStatus(jenkinsURL, token string) (*JobStatus, int, error) {
	apiURL := jenkinsURL + "/api/json?tree=building,result"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", "Basic "+token)

	client := &http.Client{Timeout: 30 * time.Second}
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
