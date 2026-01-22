package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/pidfile"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func signalDaemonReload() bool {
	if pid, running := pidfile.IsDaemonRunning(); running {
		if process, err := os.FindProcess(pid); err == nil {
			fmt.Println("Signaling daemon to reload config...")
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

	fmt.Println("Daemon not running. Starting background monitor...")
	cmd := exec.Command(os.Args[0], "_start_daemon")
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
