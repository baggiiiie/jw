package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"jenkins-monitor/pkg/ui"

	"github.com/spf13/cobra"
)

const (
	nativeHostName = "com.jw.monitor"
	extensionID    = "njbfammdojdlkbhihjiiedogedglkonc"
)

var extensionCmd = &cobra.Command{
	Use:   "extension",
	Short: "Manage the jw Chrome extension",
}

var extensionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Chrome native messaging host",
	Run:   runExtensionInstall,
}

func init() {
	extensionCmd.AddCommand(extensionInstallCmd)
	RootCmd.AddCommand(extensionCmd)
}

type nativeHostManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func runExtensionInstall(cmd *cobra.Command, args []string) {
	// 1. Resolve absolute path to jw binary
	exe, err := os.Executable()
	if err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error finding executable: %v", err)))
		os.Exit(1)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error resolving symlinks: %v", err)))
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error finding home directory: %v", err)))
		os.Exit(1)
	}

	// 2. Write wrapper script
	jwDir := filepath.Join(home, ".jw")
	if err := os.MkdirAll(jwDir, 0o755); err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error creating directory: %v", err)))
		os.Exit(1)
	}

	wrapperPath := filepath.Join(jwDir, "native-messaging-host.sh")
	wrapperContent := fmt.Sprintf("#!/bin/sh\nexec %s _native_messaging\n", exe)
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0o755); err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error writing wrapper script: %v", err)))
		os.Exit(1)
	}

	// 3. Write native messaging host manifest
	manifestDir := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error creating manifest directory: %v", err)))
		os.Exit(1)
	}

	manifest := nativeHostManifest{
		Name:           nativeHostName,
		Description:    "Jenkins job monitor - jw",
		Path:           wrapperPath,
		Type:           "stdio",
		AllowedOrigins: []string{fmt.Sprintf("chrome-extension://%s/", extensionID)},
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error marshaling manifest: %v", err)))
		os.Exit(1)
	}

	manifestPath := filepath.Join(manifestDir, nativeHostName+".json")
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		fmt.Println(ui.RedText(fmt.Sprintf("Error writing manifest: %v", err)))
		os.Exit(1)
	}

	// 4. Print success
	fmt.Println(ui.GreenText("Native messaging host installed successfully!"))
	fmt.Println()
	fmt.Printf("  Wrapper script: %s\n", wrapperPath)
	fmt.Printf("  Host manifest:  %s\n", manifestPath)
	fmt.Println()
	// Determine extension path: prefer Homebrew share dir, fall back to relative
	extensionPath := "extension/"
	if out, err := exec.Command("brew", "--prefix").Output(); err == nil {
		candidate := filepath.Join(strings.TrimSpace(string(out)), "share", "jw", "extension")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			extensionPath = candidate
		}
	}

	fmt.Println("To load the Chrome extension:")
	fmt.Println("  1. Open chrome://extensions")
	fmt.Println("  2. Enable 'Developer mode'")
	fmt.Println("  3. Copy extension to your desired location:")
	fmt.Printf("       `cp -r %s /path/to/extension`\n", extensionPath)
	fmt.Println("  4. Click 'Load unpacked' and select extension")
	fmt.Println("  5. Right-click on any Jenkins page and select 'Watch with jw'")
}
