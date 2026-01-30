package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/pidfile"
)

func signalDaemonReload() bool {
	if pid, running := pidfile.IsDaemonRunning(); running {
		if process, err := os.FindProcess(pid); err == nil {
			log.Println("Signaling daemon to reload config...")
			process.Signal(syscall.SIGHUP)
			return true
		}
	}
	return false
}

func ensureDaemonRunning() {
	if _, running := pidfile.IsDaemonRunning(); running {
		return
	}

	// Double-check: Is the daemon actually running but the PID file is missing?
	if pid, running := pidfile.FindDaemonProcess(); running {
		fmt.Println(color.YellowText(fmt.Sprintf("Daemon process found (PID: %d) but PID file is missing.", pid)))
		fmt.Println("Waiting for daemon to self-heal...")

		// Wait up to 6 seconds for the daemon's self-healing loop (5s interval) to restore the file
		for range 7 {
			time.Sleep(1 * time.Second)
			if _, running := pidfile.IsDaemonRunning(); running {
				fmt.Println(color.GreenText("Daemon PID file restored."))
				return
			}
		}

		fmt.Println(color.RedText("Daemon failed to restore PID file. It might be stuck or unresponsive."))
		fmt.Println(color.RedText("Please kill the orphaned process manually: kill " + fmt.Sprint(pid)))
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
		fmt.Println(color.RedText(fmt.Sprintf("Failed to start daemon: %v", err)))
		os.Exit(1)
	}

	// Give it a moment to start and write its PID file
	time.Sleep(1 * time.Second)

	if _, running := pidfile.IsDaemonRunning(); running {
		fmt.Println(color.GreenText("Daemon started successfully."))
	} else {
		fmt.Println(color.RedText("Failed to start daemon. Check logs for details."))
	}
}
