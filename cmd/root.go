package cmd

import (
	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/upgrade"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "jw",
	Short: "A Go-based Jenkins job monitor daemon",
	Long:  `A daemon that monitors Jenkins jobs in the background and sends macOS notifications upon completion.`,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err == nil {
			upgrade.RunCheck(cfg)
		}
	},
}
