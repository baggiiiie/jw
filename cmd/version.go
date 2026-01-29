package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/version"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\n", version.GetVersion())
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
