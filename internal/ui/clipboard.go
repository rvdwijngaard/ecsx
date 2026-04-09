package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

func copyToClipboard(s string) error {
	for _, name := range []string{"wl-copy", "xclip", "pbcopy"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path)
		if name == "xclip" {
			cmd.Args = append(cmd.Args, "-selection", "clipboard")
		}
		cmd.Stdin = strings.NewReader(s)
		return cmd.Run()
	}
	return fmt.Errorf("no clipboard tool found (install wl-copy, xclip, or pbcopy)")
}
