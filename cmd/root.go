package cmd

import (
	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/upgrade"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "jenkins-monitor",
	Short: "A Go-based Jenkins job monitor daemon",
	Long: `A daemon that monitors Jenkins jobs in the background and sends macOS notifications upon completion.
This is a Go port of the original Python script.`,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err == nil {
			upgrade.RunCheck(cfg)
		}
	},
}
