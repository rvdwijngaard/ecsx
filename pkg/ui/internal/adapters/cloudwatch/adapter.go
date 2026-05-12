// Package cloudwatch adapts CloudWatch connector responses for UI display.
package cloudwatch

import (
	"context"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	adaptypes "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatch/types"
)

// GetServiceMetrics retrieves CPU and memory utilization for an ECS service over the last hour.
func GetServiceMetrics(client *cw.Client, ctx context.Context, cluster, service string) (*adaptypes.ServiceMetrics, error) {
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	dims := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String(cluster)},
		{Name: aws.String("ServiceName"), Value: aws.String(service)},
	}

	cpu, err := getMetricStats(client, ctx, "CPUUtilization", dims, start, now)
	if err != nil {
		return nil, err
	}
	mem, err := getMetricStats(client, ctx, "MemoryUtilization", dims, start, now)
	if err != nil {
		return nil, err
	}

	m := &adaptypes.ServiceMetrics{}
	m.CPU = toSeries(cpu.Datapoints)
	m.Mem = toSeries(mem.Datapoints)
	m.HasData = len(m.CPU.Avg) > 0 || len(m.Mem.Avg) > 0
	return m, nil
}

func toSeries(dps []cwtypes.Datapoint) adaptypes.MetricSeries {
	sort.Slice(dps, func(i, j int) bool {
		return dps[i].Timestamp.Before(*dps[j].Timestamp)
	})
	s := adaptypes.MetricSeries{
		Min: make([]adaptypes.MetricPoint, 0, len(dps)),
		Max: make([]adaptypes.MetricPoint, 0, len(dps)),
		Avg: make([]adaptypes.MetricPoint, 0, len(dps)),
	}
	for _, dp := range dps {
		if dp.Timestamp == nil {
			continue
		}
		t := *dp.Timestamp
		if dp.Minimum != nil {
			s.Min = append(s.Min, adaptypes.MetricPoint{Time: t, Value: *dp.Minimum})
		}
		if dp.Maximum != nil {
			s.Max = append(s.Max, adaptypes.MetricPoint{Time: t, Value: *dp.Maximum})
		}
		if dp.Average != nil {
			s.Avg = append(s.Avg, adaptypes.MetricPoint{Time: t, Value: *dp.Average})
		}
	}
	return s
}

func getMetricStats(client *cw.Client, ctx context.Context, metricName string, dims []cwtypes.Dimension, start, end time.Time) (*cw.GetMetricStatisticsOutput, error) {
	return client.GetMetricStatistics(ctx, &cw.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/ECS"),
		MetricName: aws.String(metricName),
		Dimensions: dims,
		StartTime:  &start,
		EndTime:    &end,
		Period:     aws.Int32(60),
		Statistics: []cwtypes.Statistic{
			cwtypes.StatisticMinimum,
			cwtypes.StatisticMaximum,
			cwtypes.StatisticAverage,
		},
	})
}
