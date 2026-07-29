package ui

import (
	"encoding/json"
	"fmt"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
)

// startExecSession suspends the TUI and launches session-manager-plugin with
// the session JSON returned by the ECS ExecuteCommand API. When the session
// ends, the TUI resumes.
//
// session-manager-plugin expects positional arguments:
//
//	session-manager-plugin <session-json> <region> StartSession <profile> <target-json> <endpoint>
//
// Profile and endpoint are optional. Target JSON must include a "Target" field
// (the ECS task ARN for exec sessions).
func startExecSession(sessionJSON, region, profile, target string) tea.Cmd {
	args := []string{sessionJSON, region, "StartSession"}
	if profile != "" {
		args = append(args, profile)
	}
	if target != "" {
		targetJSON, err := json.Marshal(map[string]string{"Target": target})
		if err != nil {
			return func() tea.Msg {
				return messages.ExecFinishedMsg{Err: fmt.Errorf("marshalling target: %w", err)}
			}
		}
		args = append(args, string(targetJSON))
	}
	c := exec.Command("session-manager-plugin", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}