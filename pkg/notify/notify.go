package notify

import (
	"fmt"
	"log"
	"os/exec"
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

	var cmd [3]string
	if !m.notifierExists {
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
