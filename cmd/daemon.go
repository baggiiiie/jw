package cmd

import (
	"encoding/base64"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/logging"
	"jenkins-monitor/pkg/monitor"
	"jenkins-monitor/pkg/pidfile"

	"github.com/spf13/cobra"
)

var startDaemonCmd = &cobra.Command{
	Use:    "_start_jw_daemon",
	Short:  "Starts the daemon process (internal use)",
	Hidden: true,
	Run:    startDaemon,
}

func init() {
	RootCmd.AddCommand(startDaemonCmd)
}

// getJenkinsToken returns the base64-encoded credentials for Jenkins Basic Auth.
// It supports two modes:
// 1. JENKINS_USER + JENKINS_API_TOKEN: Combined and base64-encoded (like curl -u user:token)
// 2. JENKINS_TOKEN: Used as-is (legacy, expects pre-encoded value)
func getJenkinsToken() (string, error) {
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

	return "", nil
}

func handleJobFinish(jobURL string, logger *log.Logger, activeJobs map[string]chan struct{}) {
	err := config.Update(func(cfg *config.Config) error {
		// Directly delete from the map instead of calling cfg.RemoveJob()
		// to avoid deadlock since config.Update already holds the mutex.
		delete(cfg.Jobs, jobURL)
		return nil
	})
	if err != nil {
		logger.Printf("Error removing finished job from config: %v", err)
	}

	if stopChan, exists := activeJobs[jobURL]; exists {
		delete(activeJobs, jobURL)
		close(stopChan)
	}
}

func reloadConfigAndJobs(token string, logger *log.Logger, activeJobs map[string]chan struct{}, jobFinishedChan chan<- string) {
	reloadedCfg, err := config.Reload()
	if err != nil {
		logger.Printf("Error reloading config: %v", err)
		return
	}

	currentConfigJobs := reloadedCfg.GetJobs()

	for jobURL, stopChan := range activeJobs {
		if _, exists := currentConfigJobs[jobURL]; !exists {
			logger.Printf("Stopping monitoring for removed job: %s", jobURL)
			delete(activeJobs, jobURL)
			close(stopChan)
		}
	}

	for jobURL := range currentConfigJobs {
		if _, running := activeJobs[jobURL]; !running {
			logger.Printf("Starting to monitor new job: %s", jobURL)
			stopChan := make(chan struct{})
			activeJobs[jobURL] = stopChan
			go monitor.MonitorJob(jobURL, token, logger, func(finishedURL string) {
				jobFinishedChan <- finishedURL
			}, stopChan)
		}
	}

	logger.Printf("Configuration reloaded. Monitoring %d jobs.", len(activeJobs))
}

func startDaemon(cmd *cobra.Command, args []string) {
	if _, running := pidfile.IsDaemonRunning(); running {
		log.Println("Daemon already running.")
		return
	}

	token, _ := getJenkinsToken()
	if token == "" {
		log.Fatalln("Jenkins credentials not set! Set JENKINS_USER and JENKINS_API_TOKEN, or JENKINS_TOKEN")
	}

	logger, err := logging.SetupLogger()
	if err != nil {
		log.Fatalf("Failed to set up logger: %v", err)
	}
	logger.Println("Daemon starting...")

	if err := pidfile.Write(); err != nil {
		logger.Fatalf("Failed to write PID file: %v", err)
	}
	defer pidfile.Remove()

	activeJobs := make(map[string]chan struct{})
	jobFinishedChan := make(chan string, 10)

	if _, err := config.Load(); err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	reloadConfigAndJobsCallback := func() {
		reloadConfigAndJobs(token, logger, activeJobs, jobFinishedChan)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	reloadConfigAndJobsCallback()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				logger.Println("SIGHUP received, reloading config...")
				reloadConfigAndJobsCallback()
			case syscall.SIGINT, syscall.SIGTERM:
				logger.Println("Shutdown signal received, stopping all monitors.")
				for jobURL, stopChan := range activeJobs {
					logger.Printf("Stopping monitor for %s", jobURL)
					close(stopChan)
				}
				time.Sleep(1 * time.Second)
				logger.Println("Daemon stopped.")
				return
			}
		case jobURL := <-jobFinishedChan:
			handleJobFinish(jobURL, logger, activeJobs)
		case <-ticker.C:
			if err := pidfile.CheckAndRestore(); err != nil {
				logger.Printf("Failed to verify/restore PID file: %v", err)
			}

			if len(activeJobs) == 0 {
				logger.Println("No more jobs to monitor. Shutting down daemon.")
				return
			}
		}
	}
}
