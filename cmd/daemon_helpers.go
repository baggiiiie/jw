package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"jenkins-monitor/pkg/pidfile"
	"jenkins-monitor/pkg/ui"
)

func signalDaemonReload() bool {
	pid := ensureDaemonRunning()
	if process, err := os.FindProcess(pid); err == nil {
		process.Signal(syscall.SIGHUP)
		return true
	}
	return false
}

// startDaemonIfNeeded starts the daemon if it's not running and returns an error
// instead of printing to stdout or exiting. Suitable for contexts where stdout
// is not available (e.g., native messaging).
func startDaemonIfNeeded() error {
	if _, running := pidfile.IsDaemonRunning(); running {
		return nil
	}

	cmd := exec.Command(os.Args[0], "_start_jw_daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.ExtraFiles = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	time.Sleep(1 * time.Second)

	if _, running := pidfile.IsDaemonRunning(); !running {
		return fmt.Errorf("daemon failed to start")
	}
	return nil
}

func ensureDaemonRunning() int {
	if pid, running := pidfile.IsDaemonRunning(); running {
		return pid
	}

	// Double-check: Is the daemon actually running but the PID file is missing?
	if pid, running := pidfile.FindDaemonProcess(); running {
		fmt.Println(ui.YellowText(fmt.Sprintf("Daemon process found (PID: %d) but PID file is missing.", pid)))
		fmt.Println("Waiting for daemon to self-heal...")

		for range 7 {
			time.Sleep(1 * time.Second)
			if pid, running := pidfile.IsDaemonRunning(); running {
				fmt.Println(ui.GreenText("Daemon PID file restored."))
				return pid
			}
		}

		fmt.Println(ui.RedText("Daemon failed to restore PID file. It might be stuck or unresponsive."))
		fmt.Println(ui.RedText("Please kill the orphaned process manually: kill " + fmt.Sprint(pid)))
		os.Exit(1)
	}

	fmt.Println("Daemon not running. Starting background monitor...")
	if err := startDaemonIfNeeded(); err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Failed to start daemon: %v", err)))
		os.Exit(1)
	}

	pid, _ := pidfile.IsDaemonRunning()
	fmt.Println(ui.GreenText("Daemon started successfully."))
	return pid
}
