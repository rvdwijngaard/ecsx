package aws

import (
	"context"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type ServiceMetrics struct {
	CPU     []float64 // time-ordered avg values
	Mem     []float64
	HasData bool
}

func (c *Client) GetServiceMetrics(ctx context.Context, cluster, service string) (*ServiceMetrics, error) {
	if c.cw == nil {
		return &ServiceMetrics{}, nil
	}
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	dims := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String(cluster)},
		{Name: aws.String("ServiceName"), Value: aws.String(service)},
	}

	cpu, err := c.getMetricStats(ctx, "CPUUtilization", dims, start, now)
	if err != nil {
		return nil, err
	}
	mem, err := c.getMetricStats(ctx, "MemoryUtilization", dims, start, now)
	if err != nil {
		return nil, err
	}

	m := &ServiceMetrics{}
	m.CPU = sortedAvgs(cpu.Datapoints)
	m.Mem = sortedAvgs(mem.Datapoints)
	m.HasData = len(m.CPU) > 0 || len(m.Mem) > 0
	return m, nil
}

func sortedAvgs(dps []cwtypes.Datapoint) []float64 {
	sort.Slice(dps, func(i, j int) bool {
		return dps[i].Timestamp.Before(*dps[j].Timestamp)
	})
	vals := make([]float64, 0, len(dps))
	for _, dp := range dps {
		if dp.Average != nil {
			vals = append(vals, *dp.Average)
		}
	}
	return vals
}

func (c *Client) getMetricStats(ctx context.Context, metricName string, dims []cwtypes.Dimension, start, end time.Time) (*cw.GetMetricStatisticsOutput, error) {
	return c.cw.GetMetricStatistics(ctx, &cw.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/ECS"),
		MetricName: aws.String(metricName),
		Dimensions: dims,
		StartTime:  &start,
		EndTime:    &end,
		Period:     aws.Int32(300),
		Statistics: []cwtypes.Statistic{cwtypes.StatisticAverage},
	})
}
