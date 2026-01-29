package pidfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func GetPidFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".jenkins_monitor.pid"), nil
}

func IsDaemonRunning() (int, bool) {
	path, err := GetPidFilePath()
	if err != nil {
		return 0, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}

	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return pid, true
	}

	// If the error is ESRCH, the process doesn't exist.
	if err.(syscall.Errno) == syscall.ESRCH {
		// Clean up stale PID file
		os.Remove(path)
		return 0, false
	}

	// For other errors, we can't be sure, but we'll assume it's not running.
	return 0, false
}

func Write() error {
	path, err := GetPidFilePath()
	if err != nil {
		return err
	}

	pid := os.Getpid()
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

func Remove() error {
	path, err := GetPidFilePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// CheckAndRestore ensures the PID file exists and contains the current PID.
// If the file is missing or contains a different PID, it is overwritten.
func CheckAndRestore() error {
	path, err := GetPidFilePath()
	if err != nil {
		return err
	}

	currentPid := os.Getpid()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File is missing, restore it
			return Write()
		}
		return err
	}

	savedPid, err := strconv.Atoi(string(data))
	if err != nil || savedPid != currentPid {
		// Content is invalid or PID mismatch, restore it
		return Write()
	}

	return nil
}

// FindDaemonProcess attempts to find a running daemon process by inspecting
// the process list for the signature argument "_start_jw_daemon".
func FindDaemonProcess() (int, bool) {
	// Using pgrep to find the process with the specific argument
	cmd := exec.Command("pgrep", "-f", "_start_jw_daemon")
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}

	pids := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err == nil {
			// Ensure we don't accidentally match ourselves if we were somehow
			// called with that argument (unlikely for the CLI tool, but good practice)
			if pid != os.Getpid() {
				return pid, true
			}
		}
	}
	return 0, false
}
