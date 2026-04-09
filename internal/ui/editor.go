package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	tea "charm.land/bubbletea/v2"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type editorFinishedMsg struct {
	err  error
	path string
}

func openLogsInEditor(lines []ecsaws.LogEvent) tea.Cmd {
	f, err := os.CreateTemp("", "ecsx-logs-*.log")
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	for _, ev := range lines {
		fmt.Fprintf(f, "%s %s %s\n", ev.Timestamp.Local().Format("2006-01-02 15:04:05.000"), ev.Stream, ansiRe.ReplaceAllString(ev.Message, ""))
	}
	f.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, f.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, path: f.Name()}
	})
}
