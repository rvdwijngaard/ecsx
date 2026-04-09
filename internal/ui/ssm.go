package ui

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

type ssmFinishedMsg struct{ err error }

func startSSMSession(instanceID, region, profile string) tea.Cmd {
	args := []string{"ssm", "start-session", "--target", instanceID, "--region", region}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	c := exec.Command("aws", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ssmFinishedMsg{err: err}
	})
}
