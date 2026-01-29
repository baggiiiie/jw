package cmd

import (
	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/upgrade"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:    "_upgrade",
	Short:  "check upgrade",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			panic(err)
		}
		upgrade.RunCheck(cfg)
	},
}

func init() {
	RootCmd.AddCommand(upgradeCmd)
}
