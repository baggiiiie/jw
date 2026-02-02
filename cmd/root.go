package cmd

import (
	"time"

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
		shouldCheck := time.Since(cfg.UpgradeState.LastChecked) > 24*time.Hour
		if err == nil && shouldCheck {
			upgrade.RunCheck(cfg)
		}
	},
}
