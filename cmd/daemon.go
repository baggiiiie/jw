package cmd

import (
	"log"
	"os"
	"os/signal"
	"sync"
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

func startDaemon(cmd *cobra.Command, args []string) {
	if _, running := pidfile.IsDaemonRunning(); running {
		log.Println("Daemon already running.")
		return
	}

	token := os.Getenv("JENKINS_TOKEN")
	if token == "" {
		log.Fatalln("JENKINS_TOKEN environment variable not set!")
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

	// daemon is the single consumer of config, we implement mutex at the
	// callee-level
	var mu sync.Mutex

	// onJobFinish loads config, removes job from config, and saves file
	onJobFinish := func(jobURL string) {
		mu.Lock()
		defer mu.Unlock()

		cfg, err := config.Load()
		if err != nil {
			logger.Printf("Error loading config to remove finished job: %v", err)
			return
		}
		cfg.RemoveJob(jobURL)
		if err := cfg.Save(); err != nil {
			logger.Printf("Error saving config after removing finished job: %v", err)
		}

		if stopChan, exists := activeJobs[jobURL]; exists {
			delete(activeJobs, jobURL)
			close(stopChan)
		}
	}

	reloadConfigAndJobs := func() {
		mu.Lock()
		defer mu.Unlock()

		cfg, err := config.Load()
		if err != nil {
			logger.Printf("Error loading config: %v", err)
			return
		}

		currentConfigJobs := make(map[string]bool)
		for _, job := range cfg.Jobs {
			currentConfigJobs[job.URL] = true
		}

		for jobURL, stopChan := range activeJobs {
			if !currentConfigJobs[jobURL] {
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
				go monitor.MonitorJob(jobURL, token, logger, onJobFinish, stopChan)
			}
		}
		logger.Printf("Configuration reloaded. Monitoring %d jobs.", len(activeJobs))
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	reloadConfigAndJobs()

	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				logger.Println("SIGHUP received, reloading config...")
				reloadConfigAndJobs()
			case syscall.SIGINT, syscall.SIGTERM:
				logger.Println("Shutdown signal received, stopping all monitors.")
				mu.Lock()
				for jobURL, stopChan := range activeJobs {
					logger.Printf("Stopping monitor for %s", jobURL)
					close(stopChan)
				}
				mu.Unlock()
				time.Sleep(1 * time.Second)
				logger.Println("Daemon stopped.")
				return
			}
		case <-time.After(5 * time.Second):
			// Periodically ensure PID file exists (self-healing)
			if err := pidfile.CheckAndRestore(); err != nil {
				logger.Printf("Failed to verify/restore PID file: %v", err)
			}

			mu.Lock()
			cfg, _ := config.Load()
			if len(cfg.Jobs) == 0 {
				logger.Println("No more jobs to monitor. Shutting down daemon.")
				mu.Unlock()
				return
			}
			mu.Unlock()
		}
	}
}

func init() {
	RootCmd.AddCommand(startDaemonCmd)
}
