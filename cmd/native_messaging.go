package cmd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"jenkins-monitor/pkg/config"
	"jenkins-monitor/pkg/pidfile"

	"github.com/spf13/cobra"
)

// pidfileIsDaemonRunning wraps pidfile.IsDaemonRunning for testability.
var pidfileIsDaemonRunning = pidfile.IsDaemonRunning

type nativeRequest struct {
	URL string `json:"url"`
}

type nativeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

var nativeMessagingCmd = &cobra.Command{
	Use:    "_native_messaging",
	Short:  "Chrome native messaging host (internal use)",
	Hidden: true,
	Run:    runNativeMessaging,
}

func init() {
	RootCmd.AddCommand(nativeMessagingCmd)
}

func runNativeMessaging(cmd *cobra.Command, args []string) {
	msg, err := readNativeMessage(os.Stdin)
	if err != nil {
		writeNativeResponse(os.Stdout, nativeResponse{Error: fmt.Sprintf("failed to read message: %v", err)})
		return
	}

	var req nativeRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		writeNativeResponse(os.Stdout, nativeResponse{Error: fmt.Sprintf("invalid JSON: %v", err)})
		return
	}

	resp := handleNativeAdd(req.URL)
	writeNativeResponse(os.Stdout, resp)
}

func handleNativeAdd(jobURL string) nativeResponse {
	if _, err := config.GetCredentials(); err != nil {
		return nativeResponse{Error: err.Error()}
	}

	if !strings.HasPrefix(jobURL, "http://") && !strings.HasPrefix(jobURL, "https://") {
		return nativeResponse{Error: "URL must start with http:// or https://"}
	}

	store := config.NewDiskStore()
	var already bool
	if err := store.Update(func(cfg *config.Config) error {
		if cfg.HasJob(jobURL) {
			already = true
			return nil
		}
		cfg.AddJob(jobURL)
		return nil
	}); err != nil {
		return nativeResponse{Error: fmt.Sprintf("failed to update config: %v", err)}
	}

	if already {
		return nativeResponse{Success: true, Message: "Job is already being monitored"}
	}

	// Start the daemon if it's not running, then signal it to pick up the new job.
	if err := startDaemonIfNeeded(); err != nil {
		return nativeResponse{Success: true, Message: "Job added but daemon failed to start: " + err.Error()}
	}
	if pid, running := pidfileIsDaemonRunning(); running {
		if process, err := os.FindProcess(pid); err == nil {
			process.Signal(syscall.SIGHUP)
		}
	}

	return nativeResponse{Success: true, Message: "Job added: " + jobURL}
}

// readNativeMessage reads a Chrome native messaging protocol message from r.
// Format: 4-byte little-endian length prefix followed by JSON payload.
func readNativeMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("reading message length: %w", err)
	}

	if length > 1024*1024 {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	msg := make([]byte, length)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, fmt.Errorf("reading message body: %w", err)
	}

	return msg, nil
}

// writeNativeResponse writes a Chrome native messaging protocol response to w.
func writeNativeResponse(w io.Writer, resp nativeResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	binary.Write(w, binary.LittleEndian, uint32(len(data)))
	w.Write(data)
}
