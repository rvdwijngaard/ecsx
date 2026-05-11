package cloudwatchlogs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	logstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	apitypes "github.com/ron/ecsx/pkg/aws/cloudwatchlogs/types"
)

type tailClient interface {
	DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	StartLiveTail(context.Context, *cloudwatchlogs.StartLiveTailInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartLiveTailOutput, error)
}

// TailLogs starts a CloudWatch Live Tail session for the given log group.
// Cancel the context to stop the session. Filter pattern uses CloudWatch filter syntax.
func TailLogs(client tailClient, ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string) (<-chan apitypes.LogEvent, error) {
	// StartLiveTail requires the full log group ARN
	descOut, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: &logGroup,
	})
	if err != nil {
		return nil, fmt.Errorf("describing log group: %w", err)
	}
	var logGroupARN string
	for _, lg := range descOut.LogGroups {
		if aws.ToString(lg.LogGroupName) == logGroup {
			logGroupARN = aws.ToString(lg.Arn)
			break
		}
	}
	if logGroupARN == "" {
		return nil, fmt.Errorf("log group %s not found", logGroup)
	}
	// Some APIs return ARN with trailing :*, strip it
	logGroupARN = strings.TrimSuffix(logGroupARN, ":*")

	input := &cloudwatchlogs.StartLiveTailInput{
		LogGroupIdentifiers: []string{logGroupARN},
	}
	if logStreamPrefix != "" {
		input.LogStreamNamePrefixes = []string{logStreamPrefix}
	}
	if filterPattern != "" {
		input.LogEventFilterPattern = &filterPattern
	}

	resp, err := client.StartLiveTail(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("starting live tail (group=%s, prefix=%q, filter=%q): %w", logGroupARN, logStreamPrefix, filterPattern, err)
	}

	ch := make(chan apitypes.LogEvent, 500)
	stream := resp.GetStream()

	go func() {
		defer close(ch)
		defer stream.Close()
		events := stream.Events()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				switch e := event.(type) {
				case *logstypes.StartLiveTailResponseStreamMemberSessionUpdate:
					for _, r := range e.Value.SessionResults {
						ts := time.UnixMilli(*r.Timestamp)
						select {
						case ch <- apitypes.LogEvent{
							Timestamp: ts,
							Message:   strings.TrimRight(*r.Message, "\n"),
							Stream:    *r.LogStreamName,
							LogGroup:  aws.ToString(r.LogGroupIdentifier),
						}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()
	return ch, nil
}
