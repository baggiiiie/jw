// Package cmd implements the command-line interface for jw
package cmd

import (
	"fmt"
	"os"

	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [job_url]",
	Short: "Remove a Jenkins job from monitoring",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store := config.NewDiskStore()
		cfg, err := store.Load()
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

		if err := store.Save(cfg); err != nil {
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
