package logging

import (
	"log"
	"os"
	"path/filepath"
)

func GetLogFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".jenkins_monitor.log"), nil
}

func SetupLogger() (*log.Logger, error) {
	path, err := GetLogFilePath()
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return log.New(file, "", log.LstdFlags), nil
}
