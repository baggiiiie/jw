package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoCredentials is returned when no Jenkins credentials are configured.
var ErrNoCredentials = errors.New("Jenkins credentials not set.\n- run 'jw auth'\n- or set JENKINS_USER and JENKINS_API_TOKEN, or JENKINS_TOKEN, manually")

const credentialsFileName = ".credentials"

type Credentials struct {
	Username string `json:"username,omitempty"`
	Token    string `json:"token"`
}

// GetCredentials returns the base64-encoded credentials for Jenkins Basic Auth.
// It supports two modes:
// 1. JENKINS_USER + JENKINS_API_TOKEN: Combined and base64-encoded (like curl -u user:token)
// 2. JENKINS_TOKEN: Used as-is (legacy, expects pre-encoded value)
// 3. ~/.jw/.credentials file: Checks for stored credentials if env vars are missing
func GetCredentials() (string, error) {
	// 1. Check environment variables first (highest priority)
	user := os.Getenv("JENKINS_USER")
	apiToken := os.Getenv("JENKINS_API_TOKEN")

	if user != "" && apiToken != "" {
		credentials := user + ":" + apiToken
		return base64.StdEncoding.EncodeToString([]byte(credentials)), nil
	}

	token := os.Getenv("JENKINS_TOKEN")
	if token != "" {
		return token, nil
	}

	// 2. Check credentials file
	creds, err := LoadCredentials()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("loading credentials file: %w", err)
	}
	if creds != nil && creds.Username != "" && creds.Token != "" {
		credentials := creds.Username + ":" + creds.Token
		return base64.StdEncoding.EncodeToString([]byte(credentials)), nil
	}
	if creds != nil && creds.Token != "" {
		return creds.Token, nil
	}

	return "", ErrNoCredentials
}

func GetCredentialsPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, credentialsFileName), nil
}

func LoadCredentials() (*Credentials, error) {
	path, err := GetCredentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func SaveCredentials(creds *Credentials) error {
	path, err := GetCredentialsPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	// Save with strict permissions, as these are sensitive
	return os.WriteFile(path, data, 0o600)
}
