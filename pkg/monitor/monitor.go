package monitor

import (
	"log"
	"strings"
	"time"

	"jenkins-monitor/pkg/jenkins"
)

const pollingInterval = 30 * time.Second

// EventKind describes what happened during a monitoring check.
type EventKind int

const (
	EventStatusChecked EventKind = iota // routine status update
	EventFinished                       // job completed (SUCCESS/FAILURE/ABORTED)
	EventNotFound                       // job returned 404
	EventError                          // transient error polling
)

// JobEvent is emitted by MonitorJob to report status changes.
type JobEvent struct {
	JobURL  string
	JobName string
	Kind    EventKind
	Result  string // Jenkins result (SUCCESS, FAILURE, ABORTED) — set on EventFinished
	Failed  bool   // whether the last check failed (for config tracking)
	Error   error  // set on EventError/EventNotFound
}

// MonitorJob polls a Jenkins job for its status and emits events on the provided channel.
func MonitorJob(jobURL, token string, logger *log.Logger, events chan<- JobEvent, pollInterval time.Duration, stop <-chan struct{}) {
	if pollInterval <= 0 {
		pollInterval = pollingInterval
	}

	jobName := strings.Split(jobURL, "/job/")
	jobNameSafe := jobName[len(jobName)-1]

	logger.Printf("Started monitoring: %s", jobNameSafe)
	defer logger.Printf("Stopped monitoring: %s", jobNameSafe)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Perform the first check immediately.
	if checkJobStatus(jobURL, token, jobNameSafe, logger, events) {
		return
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if checkJobStatus(jobURL, token, jobNameSafe, logger, events) {
				return
			}
		}
	}
}

// checkJobStatus checks a Jenkins job's status and returns true if monitoring should stop.
func checkJobStatus(jobURL, token, jobNameSafe string, logger *log.Logger, events chan<- JobEvent) (shouldStop bool) {
	status, _, err := jenkins.GetJobStatus(jobURL, token)
	if err != nil {
		return handleJobStatusError(err, jobURL, jobNameSafe, logger, events)
	}

	logger.Printf("Received status for %s: Building=%v, Result=%s", jobNameSafe, status.Building, status.Result)

	isFinished := !status.Building && isFinalStatus(status.Result)
	if isFinished {
		logger.Printf("Build finished: %s - Status: %s", jobNameSafe, status.Result)
		events <- JobEvent{
			JobURL:  jobURL,
			JobName: jobNameSafe,
			Kind:    EventFinished,
			Result:  status.Result,
			Failed:  false,
		}
		return true
	}

	events <- JobEvent{
		JobURL:  jobURL,
		JobName: jobNameSafe,
		Kind:    EventStatusChecked,
		Failed:  status.Result == "FAILURE",
	}
	return false
}

// handleJobStatusError handles errors from getting job status and returns true if monitoring should stop.
func handleJobStatusError(err error, jobURL, jobNameSafe string, logger *log.Logger, events chan<- JobEvent) (shouldStop bool) {
	if strings.Contains(err.Error(), "404") {
		logger.Printf("Job '%s' not found (404). Removing.", jobNameSafe)
		events <- JobEvent{
			JobURL:  jobURL,
			JobName: jobNameSafe,
			Kind:    EventNotFound,
			Failed:  true,
			Error:   err,
		}
		return true
	}

	logger.Printf("Error getting status for %s: %v. Will retry.", jobNameSafe, err)
	events <- JobEvent{
		JobURL:  jobURL,
		JobName: jobNameSafe,
		Kind:    EventError,
		Failed:  true,
		Error:   err,
	}
	return false
}

// isFinalStatus returns true if the Jenkins build status is a final one.
func isFinalStatus(result string) bool {
	return result == "SUCCESS" || result == "FAILURE" || result == "ABORTED"
}
