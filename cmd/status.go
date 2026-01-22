package cmd

import (
	"fmt"
	"os"

	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/pidfile"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Get the status of the jenkins-monitor daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if pid, running := pidfile.IsDaemonRunning(); running {
			fmt.Println(color.GreenText(fmt.Sprintf("Daemon running (PID: %d)", pid)))
		} else {
			fmt.Println(color.RedText("Daemon not running."))
			return
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		if len(cfg.Jobs) == 0 {
			fmt.Println("Not monitoring any jobs.")
		} else {
			fmt.Printf("Monitoring %d job(s):\n", len(cfg.Jobs))
			for _, job := range cfg.Jobs {
				fmt.Printf("  - %s\n", job)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
