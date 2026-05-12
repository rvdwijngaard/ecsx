// Package cloudwatchlogs adapts CloudWatch Logs connector responses for UI display.
package cloudwatchlogs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	cwlconnector "github.com/ron/ecsx/pkg/aws/cloudwatchlogs"
	cwltypes "github.com/ron/ecsx/pkg/aws/cloudwatchlogs/types"
	ecsconnector "github.com/ron/ecsx/pkg/aws/ecs"
	adaptertypes "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
)

const defaultLookback = 5 * time.Minute

// ResolveLogGroups calls the ECS connector and returns available log configs for a service.
func ResolveLogGroups(ecsClient *ecs.Client, ctx context.Context, cluster, service string) ([]adaptertypes.LogConfig, error) {
	groups, err := ecsconnector.FindLogGroups(ecsClient, ctx, cluster, service)
	if err != nil {
		return nil, err
	}
	configs := make([]adaptertypes.LogConfig, len(groups))
	for i, g := range groups {
		configs[i] = adaptertypes.LogConfig{
			LogGroup:         g.LogGroup,
			StreamPrefix:     g.StreamPrefix,
			Container:        g.Container,
			LookbackDuration: defaultLookback,
		}
	}
	return configs, nil
}

// ResolveLogGroup resolves a single container's log config. If container is empty,
// returns the first available.
func ResolveLogGroup(ecsClient *ecs.Client, ctx context.Context, cluster, service, container string) (*adaptertypes.LogConfig, error) {
	group, err := ecsconnector.FindLogGroup(ecsClient, ctx, cluster, service, container)
	if err != nil {
		return nil, err
	}
	return &adaptertypes.LogConfig{
		LogGroup:         group.LogGroup,
		StreamPrefix:     group.StreamPrefix,
		Container:        group.Container,
		LookbackDuration: defaultLookback,
	}, nil
}

// FetchHistory fetches recent logs and returns formatted lines.
func FetchHistory(cwlClient *cloudwatchlogs.Client, ctx context.Context, cfg adaptertypes.LogConfig) ([]adaptertypes.FormattedLogLine, error) {
	start := time.Now().Add(-cfg.LookbackDuration)
	events, err := cwlconnector.FetchRecentLogs(cwlClient, ctx, cfg.LogGroup, cfg.StreamPrefix, cfg.FilterPattern, start, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching history for %s: %w", cfg.LogGroup, err)
	}
	lines := make([]adaptertypes.FormattedLogLine, len(events))
	for i, e := range events {
		lines[i] = FormatEvent(e)
	}
	return lines, nil
}

// StartTail starts a live tail and returns a channel of formatted lines.
// Cancel the context to stop the session.
func StartTail(cwlClient *cloudwatchlogs.Client, ctx context.Context, cfg adaptertypes.LogConfig) (<-chan adaptertypes.FormattedLogLine, error) {
	rawCh, err := cwlconnector.TailLogs(cwlClient, ctx, cfg.LogGroup, cfg.StreamPrefix, cfg.FilterPattern)
	if err != nil {
		return nil, fmt.Errorf("starting tail for %s: %w", cfg.LogGroup, err)
	}

	ch := make(chan adaptertypes.FormattedLogLine, 500)
	go func() {
		defer close(ch)
		for event := range rawCh {
			line := FormatEvent(event)
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// FormatEvent converts a raw connector LogEvent into a FormattedLogLine.
func FormatEvent(event cwltypes.LogEvent) adaptertypes.FormattedLogLine {
	ts := event.Timestamp.Local().Format("15:04:05.000")
	raw := fmt.Sprintf("%s  %s", ts, event.Message)
	return adaptertypes.FormattedLogLine{
		Timestamp: event.Timestamp,
		Stream:    event.Stream,
		Message:   event.Message,
		Raw:       raw,
	}
}
