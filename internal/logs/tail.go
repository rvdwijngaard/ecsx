package logs

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

// Options configures the Tail behavior.
type Options struct {
	Cluster    string
	Service    string
	Task       string
	Container  string
	Filter     string
	Follow     bool
	StreamName bool
	GroupName  bool
	Timestamp  bool
	EventID    bool
	Start      time.Time
	End        *time.Time
	Grep       string
}

// Tail resolves the log group for a service and streams logs to stdout.
func Tail(ctx context.Context, client ecsaws.ECSClient, opts Options) error {
	logGroup, streamPrefix, err := ecsaws.FindLogGroup(ctx, client, opts.Cluster, opts.Service, opts.Container)
	if err != nil {
		return fmt.Errorf("finding log group for %s/%s: %w", opts.Cluster, opts.Service, err)
	}
	effectivePrefix := streamPrefix
	if opts.Task != "" && streamPrefix != "" {
		effectivePrefix = streamPrefix + "/" + opts.Service + "/" + opts.Task
	}

	var grepRe *regexp.Regexp
	if opts.Grep != "" {
		grepRe, err = regexp.Compile(opts.Grep)
		if err != nil {
			return fmt.Errorf("invalid grep pattern: %w", err)
		}
	}

	if !opts.Start.IsZero() {
		events, err := client.FetchRecentLogs(ctx, logGroup, effectivePrefix, opts.Filter, opts.Start, opts.End)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: fetching history: %v\n", err)
		} else {
			for _, e := range events {
				printLogEvent(e, opts, grepRe)
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
		printLogEvent(event, opts, grepRe)
	}
	return nil
}

func printLogEvent(e ecsaws.LogEvent, opts Options, grepRe *regexp.Regexp) {
	if grepRe != nil && !grepRe.MatchString(e.Message) {
		return
	}
	var parts []string
	if opts.Timestamp {
		parts = append(parts, fmt.Sprintf("\033[90m%s\033[0m", e.Timestamp.Local().Format("2006-01-02T15:04:05.000")))
	}
	if opts.GroupName && e.LogGroup != "" {
		parts = append(parts, fmt.Sprintf("\033[33m%s\033[0m", e.LogGroup))
	}
	if opts.StreamName && e.Stream != "" {
		parts = append(parts, fmt.Sprintf("\033[36m%s\033[0m", e.Stream))
	}
	if opts.EventID && e.EventID != "" {
		parts = append(parts, fmt.Sprintf("\033[90m%s\033[0m", e.EventID))
	}
	parts = append(parts, e.Message)
	fmt.Println(strings.Join(parts, " "))
}
