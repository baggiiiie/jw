package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/pidfile"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the jenkins-monitor daemon",
	Run: func(cmd *cobra.Command, args []string) {
		pid, running := pidfile.IsDaemonRunning()
		if !running {
			fmt.Println(color.YellowText("Daemon not running."))
			return
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Println(color.RedText(fmt.Sprintf("Failed to find process: %v", err)))
			return
		}

		fmt.Printf("Stopping daemon (PID: %d)...\n", pid)
		if err := process.Signal(syscall.SIGTERM); err != nil {
			if err.Error() == "os: process already finished" {
				fmt.Println(color.GreenText("Daemon already stopped."))
				pidfile.Remove()
				return
			}
			fmt.Println(color.RedText(fmt.Sprintf("Failed to send signal: %v", err)))
			os.Exit(1)
		}

		time.Sleep(1 * time.Second)
		if _, running := pidfile.IsDaemonRunning(); !running {
			fmt.Println(color.GreenText("Daemon stopped successfully."))
		} else {
			fmt.Println(color.YellowText("Daemon is still running. It might be shutting down."))
		}
	},
}

func init() {
	RootCmd.AddCommand(stopCmd)
}
