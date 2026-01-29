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

const pollingInterval = 30 * time.Second

// MonitorJob polls a Jenkins job for its status and sends a notification when it finishes.
func MonitorJob(jobURL, token string, logger *log.Logger, onFinish func(jobURL string), stop <-chan struct{}) {
	jobName := strings.Split(jobURL, "/job/")
	jobNameSafe := jobName[len(jobName)-1]

	logger.Printf("Started monitoring: %s", jobNameSafe)
	defer logger.Printf("Stopped monitoring: %s", jobNameSafe)

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	// Perform the first check immediately.
	if checkJobStatus(jobURL, token, jobNameSafe, logger, onFinish) {
		return
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if checkJobStatus(jobURL, token, jobNameSafe, logger, onFinish) {
				return
			}
		}
	}
}

// checkJobStatus checks a Jenkins job's status and returns true if monitoring should stop.
func checkJobStatus(jobURL, token, jobNameSafe string, logger *log.Logger, onFinish func(jobURL string)) (shouldStop bool) {
	status, _, err := jenkins.GetJobStatus(jobURL, token)
	if err != nil {
		return handleJobStatusError(err, jobURL, jobNameSafe, logger, onFinish)
	}

	logger.Printf("Received status for %s: Building=%v, Result=%s", jobNameSafe, status.Building, status.Result)
	updateJobCheckStatusInConfig(jobURL, status.Result == "FAILURE", logger)

	isFinished := !status.Building && isFinalStatus(status.Result)
	if isFinished {
		handleFinishedJob(status, jobURL, jobNameSafe, logger, onFinish)
		return true
	}

	return false
}

// handleJobStatusError handles errors from getting job status and returns true if monitoring should stop.
func handleJobStatusError(err error, jobURL, jobNameSafe string, logger *log.Logger, onFinish func(jobURL string)) (shouldStop bool) {
	updateJobCheckStatusInConfig(jobURL, true, logger)

	if strings.Contains(err.Error(), "404") {
		logger.Printf("Job '%s' not found (404). Removing.", jobNameSafe)
		_ = notify.Send(
			"Jenkins Job Not Found",
			fmt.Sprintf("Job: %s\nURL returned 404. Removing from monitor.", jobNameSafe),
			jobURL,
		)
		onFinish(jobURL)
		return true // Stop monitoring for 404 errors.
	}

	logger.Printf("Error getting status for %s: %v. Will retry.", jobNameSafe, err)
	return false // Continue monitoring for other transient errors.
}

// handleFinishedJob sends a notification and cleans up a finished job.
func handleFinishedJob(status *jenkins.JobStatus, jobURL, jobNameSafe string, logger *log.Logger, onFinish func(jobURL string)) {
	logger.Printf("Build finished: %s - Status: %s", jobNameSafe, status.Result)

	notificationTitle := "Jenkins Job Completed"
	if status.Result == "FAILURE" {
		notificationTitle = "Jenkins Job Failed"
	}

	if err := notify.Send(notificationTitle, fmt.Sprintf("Job: %s\nStatus: %s", jobNameSafe, status.Result), jobURL); err != nil {
		logger.Printf("Failed to send notification: %v", err)
	} else {
		logger.Printf("Sent notification for %s", jobURL)
	}

	// Always remove finished jobs from monitoring.
	onFinish(jobURL)
}

// isFinalStatus returns true if the Jenkins build status is a final one.
func isFinalStatus(result string) bool {
	return result == "SUCCESS" || result == "FAILURE" || result == "ABORTED"
}

// updateJobCheckStatusInConfig loads the config, updates the check status for a job, and saves it.
func updateJobCheckStatusInConfig(jobURL string, failed bool, logger *log.Logger) {
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
