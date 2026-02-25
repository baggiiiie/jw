package notify

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

type Notifier interface {
	Send(title, message, url string) error
}

type MacNotifier struct {
	once           sync.Once
	notifierExists bool
}

func (m *MacNotifier) checkNotifier() {
	m.once.Do(func() {
		if _, err := exec.LookPath("terminal-notifier"); err != nil {
			m.notifierExists = false
		} else {
			m.notifierExists = true
		}
	})
}

func (m *MacNotifier) Send(title, message, url string) error {
	m.checkNotifier()

	var result *exec.Cmd
	if !m.notifierExists {
		script := fmt.Sprintf(
			`display notification (do shell script "echo %s") with title (do shell script "echo %s")`,
			shellQuote(message), shellQuote(title),
		)
		result = exec.Command("osascript", "-e", script)
		log.Println("Couldn't find 'terminal-notifier' on host")
	} else {
		args := []string{
			"-message", message,
			"-title", title,
			"-sound", "ping",
			"-group", "jenkins_monitor",
		}
		if url != "" {
			args = append(args, "-open", url)
		}
		result = exec.Command("terminal-notifier", args...)
	}

	output, err := result.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to send notification: %w (output: %s)", err, string(output))
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
