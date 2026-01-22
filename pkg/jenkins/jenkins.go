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

func GetJobStatus(jenkinsURL, token string) (*JobStatus, error) {
	apiURL := jenkinsURL + "/api/json?tree=building,result"
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Basic "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("job not found (404)")
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %s", resp.Status)
	}

	var status JobStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}
