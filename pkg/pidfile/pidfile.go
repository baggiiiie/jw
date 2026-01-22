package pidfile

import (
	"os"
	"path/filepath"
	"strconv"
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
