package cloudwatchlogs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	apitypes "github.com/rvdwijngaard/ecsx/pkg/aws/cloudwatchlogs/types"
)

type fetchClient interface {
	FilterLogEvents(context.Context, *cloudwatchlogs.FilterLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

// FetchRecentLogs retrieves historical log events from the given log group.
func FetchRecentLogs(client fetchClient, ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string, start time.Time, end *time.Time) ([]apitypes.LogEvent, error) {
	startMs := start.UnixMilli()
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &logGroup,
		StartTime:    &startMs,
	}
	if end != nil {
		endMs := end.UnixMilli()
		input.EndTime = &endMs
	}
	if logStreamPrefix != "" {
		input.LogStreamNamePrefix = &logStreamPrefix
	}
	if filterPattern != "" {
		input.FilterPattern = &filterPattern
	}
	var events []apitypes.LogEvent
	p := cloudwatchlogs.NewFilterLogEventsPaginator(client, input)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("filtering log events: %w", err)
		}
		for _, e := range page.Events {
			events = append(events, apitypes.LogEvent{
				Timestamp: time.UnixMilli(aws.ToInt64(e.Timestamp)),
				Message:   strings.TrimRight(aws.ToString(e.Message), "\n"),
				Stream:    aws.ToString(e.LogStreamName),
				EventID:   aws.ToString(e.EventId),
				LogGroup:  logGroup,
			})
		}
	}
	return events, nil
}
