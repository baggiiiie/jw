package monitor

import (
	"fmt"
	"log"
	"strings"
	"time"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/jenkins"
	"jenkins-monitor/pkg/notify"
)

func MonitorJob(jobURL, token string, logger *log.Logger, onFinish func(jobURL string), stop <-chan struct{}) {
	jobName := strings.Split(jobURL, "/job/")
	jobNameSafe := jobName[len(jobName)-1]

	logger.Printf("Started monitoring: %s", jobNameSafe)
	defer logger.Printf("Stopped monitoring: %s", jobNameSafe)

	var lastResult string
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Helper to update check status in config
	updateCheckStatus := func(failed bool) {
		cfg, err := config.Load()
		if err != nil {
			logger.Printf("Error loading config to update check status: %v", err)
			return
		}
		cfg.UpdateJobCheckStatus(jobURL, failed)
		if err := cfg.Save(); err != nil {
			logger.Printf("Error saving config with check status: %v", err)
		}
	}

	// Perform first check immediately
	checkJobStatus := func() (shouldStop bool) {
		status, _, err := jenkins.GetJobStatus(jobURL, token)
		if err != nil {
			updateCheckStatus(true)
			if strings.Contains(err.Error(), "404") {
				logger.Printf("Job '%s' not found (404). Removing.", jobNameSafe)
				notify.Send(
					"Jenkins Job Not Found",
					fmt.Sprintf("Job: %s\nURL returned 404. Removing from monitor.", jobNameSafe),
					jobURL,
				)
				onFinish(jobURL)
				return true
			}
			logger.Printf("Error getting status for %s: %v. Will retry.", jobNameSafe, err)
			// Don't stop for transient errors
			return false
		}

		updateCheckStatus(false)

		if !status.Building && status.Result != lastResult {
			result := status.Result
			if result == "" {
				result = "UNKNOWN"
			}
			logger.Printf("Build finished: %s - Status: %s", jobNameSafe, result)
			notify.Send(
				"Jenkins Job Completed",
				fmt.Sprintf("Job: %s\nStatus: %s", jobNameSafe, result),
				jobURL,
			)
			onFinish(jobURL)
			return true
		}
		lastResult = status.Result
		return false
	}

	if checkJobStatus() {
		return
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if checkJobStatus() {
				return
			}
		}
	}
}
