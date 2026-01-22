package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/logging"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Follow the logs of the jenkins-monitor daemon",
	Run: func(cmd *cobra.Command, args []string) {
		logFile, err := logging.GetLogFilePath()
		if err != nil {
			fmt.Println("Error getting log file path:", err)
			os.Exit(1)
		}

		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			fmt.Println("Log file not found:", logFile)
			return
		}

		tailCmd := exec.Command("tail", "-f", logFile)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr

		// Handle Ctrl+C
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigs
			fmt.Println("\nStopped following logs.")
			os.Exit(0)
		}()

		if err := tailCmd.Run(); err != nil {
			fmt.Println("Error tailing log file:", err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(logsCmd)
}
