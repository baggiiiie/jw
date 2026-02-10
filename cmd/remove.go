// Package cmd implements the command-line interface for jw
package cmd

import (
	"fmt"
	"os"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/ui"

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
			fmt.Println(ui.RedText(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		jobURL := args[0]
		if !cfg.HasJob(jobURL) {
			fmt.Println(ui.YellowText("Job not found in config: " + jobURL))
			return
		}

		cfg.RemoveJob(jobURL)

		if err := store.Save(cfg); err != nil {
			fmt.Println(ui.RedText(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Println(ui.GreenText("Removed job from config: " + jobURL))

		if signalDaemonReload() {
			fmt.Println("Daemon signaled to stop monitoring the job.")
		}
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}
