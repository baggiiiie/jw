package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [job_url]",
	Short: "Add a Jenkins job to monitor",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobURL := args[0]
		if !strings.HasPrefix(jobURL, "http://") && !strings.HasPrefix(jobURL, "https://") {
			fmt.Println(color.RedText("Error: Job URL must start with http:// or https://"))
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		
		if cfg.HasJob(jobURL) {
			fmt.Println(color.YellowText("Job is already being monitored: " + jobURL))
			return
		}

		cfg.AddJob(jobURL)

		if err := cfg.Save(); err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Println(color.GreenText("Added job to config: " + jobURL))

		if signalDaemonReload() {
			fmt.Println("Daemon signaled to monitor the new job.")
		} else {
			ensureDaemonRunning()
		}
	},
}

func init() {
	RootCmd.AddCommand(addCmd)
}
