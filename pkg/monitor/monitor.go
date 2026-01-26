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

		logger.Printf("Received status for %s: Building=%v, Result=%s, LastResult=%s", jobNameSafe, status.Building, status.Result, lastResult)
		updateCheckStatus(status.Result == "FAILURE")

		isFinished := !status.Building && (status.Result == "SUCCESS" || status.Result == "FAILURE" || status.Result == "ABORTED")

		if isFinished {
			// Only send notification if result changed (avoid duplicates)
			if status.Result != lastResult {
				logger.Printf("Build finished: %s - Status: %s", jobNameSafe, status.Result)

				notificationTitle := "Jenkins Job Completed"
				if status.Result == "FAILURE" {
					notificationTitle = "Jenkins Job Failed"
				}

				err := notify.Send(
					notificationTitle,
					fmt.Sprintf("Job: %s\nStatus: %s", jobNameSafe, status.Result),
					jobURL,
				)
				if err != nil {
					logger.Printf("Failed to send notification: %v", err)
				}
			} else {
				logger.Printf("Build already finished: %s - Status: %s (removing without notification)", jobNameSafe, status.Result)
			}

			// Always remove finished jobs
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
