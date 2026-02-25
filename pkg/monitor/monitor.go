package monitor

import (
	"errors"
	"log"
	"net"
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
	EventUnauthorized                   // job returned 401
	EventClientError                    // other non-transient 4xx
	EventDNSError                       // DNS resolution failed (invalid host)
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
	status, statusCode, err := jenkins.GetJobStatus(jobURL, token)
	if err != nil {
		return handleJobStatusError(err, statusCode, jobURL, jobNameSafe, logger, events)
	}

	logger.Printf("Received status for %s: Building=%v, Result=%s", jobNameSafe, status.Building, status.Result)

	if !status.Building {
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
func handleJobStatusError(err error, statusCode int, jobURL, jobNameSafe string, logger *log.Logger, events chan<- JobEvent) (shouldStop bool) {
	if statusCode == 404 {
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

	if statusCode == 401 || statusCode == 403 {
		logger.Printf("Unauthorized for job '%s' (%d). Removing.", jobNameSafe, statusCode)
		events <- JobEvent{
			JobURL:  jobURL,
			JobName: jobNameSafe,
			Kind:    EventUnauthorized,
			Failed:  true,
			Error:   err,
		}
		return true
	}

	if statusCode >= 400 && statusCode < 500 && statusCode != 429 {
		logger.Printf("Client error for job '%s' (%d). Removing.", jobNameSafe, statusCode)
		events <- JobEvent{
			JobURL:  jobURL,
			JobName: jobNameSafe,
			Kind:    EventClientError,
			Failed:  true,
			Error:   err,
		}
		return true
	}

	// DNS resolution failure means the host doesn't exist — no point retrying.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		logger.Printf("DNS lookup failed for job '%s': %v. Removing.", jobNameSafe, err)
		events <- JobEvent{
			JobURL:  jobURL,
			JobName: jobNameSafe,
			Kind:    EventDNSError,
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
