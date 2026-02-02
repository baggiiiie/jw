package upgrade

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/version"

	"golang.org/x/mod/semver"
)

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

func RunCheck(cfg *config.Config) {
	current := version.GetVersion()
	fmt.Println("Current version:", current)
	// Skip check for dev builds
	if current == "dev" {
		return
	}
	if strings.Contains(current, "-") {
		current = strings.SplitN(current, "-", 2)[0]
	}

	latest := cfg.UpgradeState.LatestVersion

	newLatest, err := fetchLatestVersion()
	if err == nil {
		latest = newLatest
		cfg.UpgradeState.LatestVersion = latest
		cfg.UpgradeState.LastChecked = time.Now()
		// Ignore save error, not critical
		_ = cfg.Save()
	}

	if latest != "" && semver.Compare(current, latest) < 0 {
		promptUpgrade(current, latest)
	}
}

func fetchLatestVersion() (string, error) {
	client := http.Client{
		Timeout: 500 * time.Millisecond,
	}

	resp, err := client.Get("https://api.github.com/repos/baggiiiie/jw/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

func promptUpgrade(current, latest string) {
	msg := fmt.Sprintf("\nNew version available: %s -> %s\nhttps://github.com/baggiiiie/jw/releases/latest\n", current, latest)
	fmt.Println(color.MutedText(msg))
}
