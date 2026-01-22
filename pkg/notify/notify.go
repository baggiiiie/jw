package notify

import (
	"fmt"
	"os/exec"
)

func Send(title, message, url string) error {
	urlCmd := ""
	if url != "" {
		urlCmd = fmt.Sprintf("-open %s", url)
	}
	cmd := fmt.Sprintf("terminal-notifier -message '%s' -title '%s' -sound ping %s -group 'jenkins_monitor'", message, title, urlCmd)
	
	result := exec.Command("sh", "-c", cmd)
	if err := result.Run(); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	return nil
}
