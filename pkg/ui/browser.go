package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"

	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
)

// openURL launches the user's default browser to display the given URL. The
// command runs without blocking the TUI.
//
// Returns a tea.Cmd that emits a ToggleNotificationDialog on error (e.g. no
// browser launcher available), or nil on success.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var bin string
		var args []string
		switch runtime.GOOS {
		case "darwin":
			bin = "open"
			args = []string{url}
		case "windows":
			bin = "rundll32"
			args = []string{"url.dll,FileProtocolHandler", url}
		default:
			bin = "xdg-open"
			args = []string{url}
		}
		cmd := exec.Command(bin, args...)
		if err := cmd.Start(); err != nil {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("open %s: %w", url, err),
			}
		}
		// Don't wait — the browser process should outlive the TUI subprocess
		// call. If we Wait(), a hung browser blocks the TUI.
		go func() { _ = cmd.Wait() }()
		return nil
	}
}