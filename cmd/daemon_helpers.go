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

func ensureDaemonRunning() int {
	if pid, running := pidfile.IsDaemonRunning(); running {
		return pid
	}

	// Double-check: Is the daemon actually running but the PID file is missing?
	if pid, running := pidfile.FindDaemonProcess(); running {
		fmt.Println(ui.YellowText(fmt.Sprintf("Daemon process found (PID: %d) but PID file is missing.", pid)))
		fmt.Println("Waiting for daemon to self-heal...")

		// Wait up to 6 seconds for the daemon's self-healing loop (5s interval) to restore the file
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
	cmd := exec.Command(os.Args[0], "_start_jw_daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.ExtraFiles = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Failed to start daemon: %v", err)))
		os.Exit(1)
	}

	// Give it a moment to start and write its PID file
	time.Sleep(1 * time.Second)

	pid, running := pidfile.IsDaemonRunning()
	if !running {
		fmt.Println(ui.RedText("Failed to start daemon. Check logs for details."))
		os.Exit(1)
	}
	fmt.Println(ui.GreenText("Daemon started successfully."))
	return pid
}
