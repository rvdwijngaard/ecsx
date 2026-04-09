package logs

import (
	"context"
	"fmt"
	"os"
	"time"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

// Options configures the Tail behavior.
type Options struct {
	Cluster    string
	Service    string
	Task       string
	Filter     string
	Follow     bool
	StreamName bool
	Start      time.Time
	End        *time.Time
}

// Tail resolves the log group for a service and streams logs to stdout.
func Tail(ctx context.Context, client ecsaws.ECSClient, opts Options) error {
	logGroup, streamPrefix, err := ecsaws.FindLogGroup(ctx, client, opts.Cluster, opts.Service)
	if err != nil {
		return fmt.Errorf("finding log group for %s/%s: %w", opts.Cluster, opts.Service, err)
	}
	effectivePrefix := streamPrefix
	if opts.Task != "" && streamPrefix != "" {
		effectivePrefix = streamPrefix + "/" + opts.Service + "/" + opts.Task
	}

	if !opts.Start.IsZero() {
		events, err := client.FetchRecentLogs(ctx, logGroup, effectivePrefix, opts.Filter, opts.Start, opts.End)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: fetching history: %v\n", err)
		} else {
			for _, e := range events {
				printLogEvent(e, opts.StreamName)
			}
		}
	}

	if !opts.Follow {
		return nil
	}

	ch, err := client.TailLogs(ctx, logGroup, effectivePrefix, opts.Filter)
	if err != nil {
		return fmt.Errorf("starting live tail: %w", err)
	}
	for event := range ch {
		printLogEvent(event, opts.StreamName)
	}
	return nil
}

func printLogEvent(e ecsaws.LogEvent, showStream bool) {
	if showStream {
		fmt.Printf("\033[90m%s\033[0m \033[36m%s\033[0m %s\n", e.Timestamp.Local().Format("15:04:05.000"), e.Stream, e.Message)
	} else {
		fmt.Printf("\033[90m%s\033[0m %s\n", e.Timestamp.Local().Format("15:04:05.000"), e.Message)
	}
}
