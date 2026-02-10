package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/jenkins"
	"jenkins-monitor/pkg/ui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Jenkins",
	Long:  `Authenticate with Jenkins by providing your username and password. This will generate an API token and save it locally.`,
	Run:   runAuth,
}

func init() {
	RootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)

	// Check if credentials already exist
	if existing, err := config.LoadCredentials(); err == nil && existing != nil {
		fmt.Printf("Credentials already exist for user %s. Refresh? [y/N]: ", ui.YellowText(existing.Username))
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return
		}
	}

	// 1. Get Jenkins URL
	fmt.Print("Enter Jenkins URL (e.g. https://jenkins.example.com): ")
	jenkinsURL, _ := reader.ReadString('\n')
	jenkinsURL = strings.TrimSpace(jenkinsURL)
	if jenkinsURL == "" {
		fmt.Println(ui.RedText("Error: Jenkins URL is required"))
		os.Exit(1)
	}
	jenkinsURL = strings.TrimRight(jenkinsURL, "/")

	// 2. Get Username
	fmt.Print("Enter Jenkins Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		fmt.Println(ui.RedText("Error: Username is required"))
		os.Exit(1)
	}

	// 3. Get Password
	fmt.Print("Enter Jenkins Password (hidden): ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println(ui.RedText("\nError reading password: " + err.Error()))
		os.Exit(1)
	}
	password := string(bytePassword)
	fmt.Println()

	// 4. Authenticate and Generate Token
	spinner := ui.NewSpinner("Fetching token")
	spinner.Start()
	newToken, err := jenkins.AuthenticateAndGenerateToken(jenkinsURL, username, password)
	spinner.Stop()

	if err != nil {
		fmt.Println(ui.RedText(err.Error()))
		os.Exit(1)
	}

	// 5. Save Credentials
	creds := &config.Credentials{
		Username: username,
		Token:    newToken,
	}

	if err := config.SaveCredentials(creds); err != nil {
		fmt.Println(ui.RedText("Error saving credentials: " + err.Error()))
		os.Exit(1)
	}

	fmt.Println(ui.GreenText("Success! Credentials saved to ~/.jw/.credentials"))
}
