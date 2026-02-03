package cmd

import (
	"fmt"
	"os"
	"strings"

	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [job_url]",
	Short: "Add a Jenkins job to monitor",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hasUserCreds := os.Getenv("JENKINS_USER") != "" && os.Getenv("JENKINS_API_TOKEN") != ""
		hasLegacyToken := os.Getenv("JENKINS_TOKEN") != ""
		if !hasUserCreds && !hasLegacyToken {
			fmt.Println(color.RedText("Error: Jenkins credentials not set. Set JENKINS_USER and JENKINS_API_TOKEN, or JENKINS_TOKEN"))
			os.Exit(1)
		}

		jobURL := args[0]
		if !strings.HasPrefix(jobURL, "http://") && !strings.HasPrefix(jobURL, "https://") {
			fmt.Println(color.RedText("Error: Job URL must start with http:// or https://"))
			os.Exit(1)
		}

		store := config.NewDiskStore()
		cfg, err := store.Load()
		if err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		if cfg.HasJob(jobURL) {
			fmt.Println(color.YellowText("Job is already being monitored: " + jobURL))
			return
		}

		cfg.AddJob(jobURL)

		if err := store.Save(cfg); err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Println(color.GreenText("Added job to config: " + jobURL))

		if signalDaemonReload() {
			fmt.Println("Daemon signaled to monitor the new job.")
		}
	},
}

func init() {
	RootCmd.AddCommand(addCmd)
}
