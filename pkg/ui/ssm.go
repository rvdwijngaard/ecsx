package ui

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
)

// startSSMSession suspends the TUI and starts an interactive AWS SSM session
// against the given EC2 instance. When the session ends, the TUI resumes.
func startSSMSession(instanceID, region, profile string) tea.Cmd {
	args := []string{"ssm", "start-session", "--target", instanceID, "--region", region}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	c := exec.Command("aws", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.SSMFinishedMsg{Err: err}
	})
}
