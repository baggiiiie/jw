package notify

import (
	"fmt"
	"log"
	"os/exec"
)

var notifierExists = checkNotifier()

func checkNotifier() bool {
	if _, err := exec.LookPath("terminal-notifier"); err != nil {
		_ = exec.Command("osascript", "-e", `display notification "install with homebrew
			for better experience" with title "terminal-notifier not found"`).Run()
		return false
	}
	return true
}

func Send(title, message, url string) error {
	var cmd [3]string
	if !notifierExists {
		cmd[0] = "osascript"
		cmd[1] = "-e"
		cmd[2] = fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		log.Println("Couldn't find 'terminal-notifier' on host")
	} else {
		cmd[0] = "sh"
		cmd[1] = "-c"
		cmd[2] = fmt.Sprintf("terminal-notifier -message '%s' -title '%s' -sound ping -group 'jenkins_monitor'", message, title)
	}

	if url != "" {
		cmd[2] = fmt.Sprintf("%s -open %s", cmd[2], url)
	}

	result := exec.Command(cmd[0], cmd[1], cmd[2])
	output, err := result.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to send notification: %w (output: %s)", err, string(output))
	}
	return nil
}
