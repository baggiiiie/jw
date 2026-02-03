package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/pidfile"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var tui bool

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Get the status of the jenkins-monitor daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if tui {
			runTUI()
			return
		}
		if pid, running := pidfile.IsDaemonRunning(); running {
			fmt.Println(color.GreenText(fmt.Sprintf("Daemon running (PID: %d)", pid)))
		} else {
			fmt.Println(color.RedText("Daemon not running."))
			return
		}

		store := config.NewDiskStore()
		cfg, err := store.Load()
		if err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		if len(cfg.Jobs) == 0 {
			fmt.Println("Not monitoring any jobs.")
		} else {
			fmt.Printf("Monitoring %d job(s):\n", len(cfg.Jobs))
			for _, job := range cfg.Jobs {
				duration := time.Since(job.StartTime)
				urlParts := strings.Split(job.URL, "/")
				url := strings.Join(urlParts[len(urlParts)-3:], "/")
				line := fmt.Sprintf("  - %s (monitored for %s)", url, formatDuration(duration))
				if job.LastCheckFailed {
					fmt.Println(color.YellowText(line))
				} else {
					fmt.Println(line)
				}
			}
		}
	},
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hrs := d / time.Hour
	d -= hrs * time.Hour
	mins := d / time.Minute
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hrs, mins)
	}
	if hrs > 0 {
		return fmt.Sprintf("%dh %dm", hrs, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func init() {
	RootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&tui, "tui", false, "Display status in a TUI table")
}
