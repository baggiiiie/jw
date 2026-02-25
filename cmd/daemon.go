package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/logging"
	"jenkins-monitor/pkg/monitor"
	"jenkins-monitor/pkg/notify"
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

func handleJobEvent(event monitor.JobEvent, logger *log.Logger, store config.ConfigStore, activeJobs map[string]chan struct{}, notifier notify.Notifier) {
	switch event.Kind {
	case monitor.EventStatusChecked, monitor.EventError:
		updateJobCheckStatus(event.JobURL, event.Failed, logger, store)

	case monitor.EventFinished:
		notificationTitle := "Jenkins Job Completed"
		if event.Result == "FAILURE" {
			notificationTitle = "Jenkins Job Failed"
		}
		if err := notifier.Send(notificationTitle, fmt.Sprintf("Job: %s\nStatus: %s", event.JobName, event.Result), event.JobURL); err != nil {
			logger.Printf("Failed to send notification: %v", err)
		} else {
			logger.Printf("Sent notification for %s", event.JobURL)
		}
		removeJob(event.JobURL, logger, store, activeJobs)

	case monitor.EventNotFound:
		_ = notifier.Send(
			"Jenkins Job Not Found",
			fmt.Sprintf("Job: %s\nURL returned 404. Removing from monitor.", event.JobName),
			event.JobURL,
		)
		removeJob(event.JobURL, logger, store, activeJobs)

	case monitor.EventUnauthorized:
		_ = notifier.Send(
			"Jenkins Auth Failed",
			fmt.Sprintf("Job: %s\nUnauthorized (401/403). Check credentials. Removing from monitor.", event.JobName),
			event.JobURL,
		)
		removeJob(event.JobURL, logger, store, activeJobs)

	case monitor.EventClientError:
		_ = notifier.Send(
			"Jenkins Request Error",
			fmt.Sprintf("Job: %s\n%v. Removing from monitor.", event.JobName, event.Error),
			event.JobURL,
		)
		removeJob(event.JobURL, logger, store, activeJobs)

	case monitor.EventDNSError:
		_ = notifier.Send(
			"Jenkins Job Unreachable",
			fmt.Sprintf("Job: %s\nDNS lookup failed — host not found. Removing from monitor.", event.JobName),
			event.JobURL,
		)
		removeJob(event.JobURL, logger, store, activeJobs)
	}
}

func updateJobCheckStatus(jobURL string, failed bool, logger *log.Logger, store config.ConfigStore) {
	err := store.Update(func(cfg *config.Config) error {
		if job, exists := cfg.Jobs[jobURL]; exists {
			if job.LastCheckFailed != failed {
				job.LastCheckFailed = failed
				cfg.Jobs[jobURL] = job
			}
		}
		return nil
	})
	if err != nil {
		logger.Printf("Error updating job check status in config: %v", err)
	}
}

func removeJob(jobURL string, logger *log.Logger, store config.ConfigStore, activeJobs map[string]chan struct{}) {
	err := store.Update(func(cfg *config.Config) error {
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

type DaemonDeps struct {
	Store          config.ConfigStore
	Notifier       notify.Notifier
	Token          string
	SigChan        <-chan os.Signal
	Stop           <-chan struct{}
	PollInterval   time.Duration
	TickerInterval time.Duration
	OnTick         func()
}

func reloadConfigAndJobs(deps DaemonDeps, logger *log.Logger, activeJobs map[string]chan struct{}, events chan<- monitor.JobEvent) {
	reloadedCfg, err := deps.Store.Load()
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
			go monitor.MonitorJob(jobURL, deps.Token, logger, events, deps.PollInterval, stopChan)
		}
	}

	logger.Printf("Configuration reloaded. Monitoring %d jobs.", len(activeJobs))
}

func runDaemonLoop(deps DaemonDeps, logger *log.Logger) error {
	if _, err := deps.Store.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	activeJobs := make(map[string]chan struct{})
	events := make(chan monitor.JobEvent, 10)

	reloadConfigAndJobs(deps, logger, activeJobs, events)

	tickerInterval := deps.TickerInterval
	if tickerInterval <= 0 {
		tickerInterval = 5 * time.Second
	}
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-deps.Stop:
			logger.Println("Stop received, stopping all monitors.")
			for jobURL, stopChan := range activeJobs {
				logger.Printf("Stopping monitor for %s", jobURL)
				close(stopChan)
			}
			logger.Println("Daemon stopped.")
			return nil

		case sig := <-deps.SigChan:
			switch sig {
			case syscall.SIGHUP:
				logger.Println("SIGHUP received, reloading config...")
				reloadConfigAndJobs(deps, logger, activeJobs, events)
			case syscall.SIGINT, syscall.SIGTERM:
				logger.Println("Shutdown signal received, stopping all monitors.")
				for jobURL, stopChan := range activeJobs {
					logger.Printf("Stopping monitor for %s", jobURL)
					close(stopChan)
				}
				time.Sleep(1 * time.Second)
				logger.Println("Daemon stopped.")
				return nil
			}

		case event := <-events:
			handleJobEvent(event, logger, deps.Store, activeJobs, deps.Notifier)

		case <-ticker.C:
			if deps.OnTick != nil {
				deps.OnTick()
			}

			if len(activeJobs) == 0 {
				logger.Println("No more jobs to monitor. Shutting down daemon.")
				return nil
			}
		}
	}
}

func startDaemon(cmd *cobra.Command, args []string) {
	if _, running := pidfile.IsDaemonRunning(); running {
		log.Println("Daemon already running.")
		return
	}

	token, err := config.GetCredentials()
	if err != nil {
		log.Fatalln(err)
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	deps := DaemonDeps{
		Store:          config.NewDiskStore(),
		Notifier:       &notify.MacNotifier{},
		Token:          token,
		SigChan:        sigChan,
		Stop:           make(chan struct{}),
		PollInterval:   0,
		TickerInterval: 5 * time.Second,
		OnTick: func() {
			if err := pidfile.CheckAndRestore(); err != nil {
				logger.Printf("Failed to verify/restore PID file: %v", err)
			}
		},
	}

	if err := runDaemonLoop(deps, logger); err != nil {
		logger.Fatalf("Daemon loop failed: %v", err)
	}
}
