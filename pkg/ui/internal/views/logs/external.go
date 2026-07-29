package logsview

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	tea "charm.land/bubbletea/v2"

	cwladapter "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs"
	adaptertypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
)

// ExternalViewerFinishedMsg is sent when the external viewer process exits.
type ExternalViewerFinishedMsg struct {
	Err error
}

// OpenInExternalViewer suspends the TUI and pipes CloudWatch logs into the
// configured external viewer (e.g. "hl -L -F"). The user Ctrl+C's to return.
func OpenInExternalViewer(
	viewerCmd string,
	ecsClient *ecs.Client,
	cwlClient *cloudwatchlogs.Client,
	cluster, service, container string,
	period time.Duration,
	filterPattern string,
) tea.Cmd {
	cmd := exec.Command("sh", "-c", "clear; "+viewerCmd)

	// Set up stdin pipe — we'll write log lines into it
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return func() tea.Msg {
			return ExternalViewerFinishedMsg{Err: fmt.Errorf("creating stdin pipe: %w", err)}
		}
	}

	// Resolve log group before handing off to tea.Exec
	ctx := context.Background()
	logCfg, err := cwladapter.ResolveLogGroup(ecsClient, ctx, cluster, service, container)
	if err != nil {
		return func() tea.Msg {
			return ExternalViewerFinishedMsg{Err: fmt.Errorf("resolving log group: %w", err)}
		}
	}

	// Apply period and filter
	if period > 0 {
		logCfg.LookbackDuration = period
	}
	logCfg.FilterPattern = filterPattern

	// Start a goroutine that streams logs into the viewer's stdin.
	// It uses a context that gets cancelled when the pipe is closed (viewer exits).
	streamCtx, streamCancel := context.WithCancel(context.Background())

	go streamLogs(streamCtx, streamCancel, stdinPipe, cwlClient, ecsClient, *logCfg)

	// tea.ExecProcess suspends the TUI and gives the terminal to the viewer.
	// When the viewer exits (Ctrl+C), the TUI resumes.
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		streamCancel()
		return ExternalViewerFinishedMsg{Err: err}
	})
}

// streamLogs fetches history then tails, writing each log message to the writer.
func streamLogs(
	ctx context.Context,
	cancel context.CancelFunc,
	w io.WriteCloser,
	cwlClient *cloudwatchlogs.Client,
	ecsClient *ecs.Client,
	logCfg adaptertypes.LogConfig,
) {
	defer w.Close()
	defer cancel()

	// Fetch history
	history, err := cwladapter.FetchHistory(cwlClient, ctx, logCfg)
	if err == nil {
		for _, line := range history {
			if ctx.Err() != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "%s\n", line.Message); err != nil {
				return
			}
		}
	}

	// Start tail
	ch, err := cwladapter.StartTail(cwlClient, ctx, logCfg)
	if err != nil {
		return
	}

	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "%s\n", line.Message); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
