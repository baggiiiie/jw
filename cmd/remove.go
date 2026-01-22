package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"
	"os"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [job_url]",
	Short: "Remove a Jenkins job from monitoring",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		jobURL := args[0]
		if !cfg.HasJob(jobURL) {
			fmt.Println(color.YellowText("Job not found in config: " + jobURL))
			return
		}

		cfg.RemoveJob(jobURL)

		if err := cfg.Save(); err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Println(color.GreenText("Removed job from config: " + jobURL))

		if signalDaemonReload() {
			fmt.Println("Daemon signaled to stop monitoring the job.")
		}
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}
