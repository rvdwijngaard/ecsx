package cloudwatch

import (
	"context"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	apitypes "github.com/rvdwijngaard/ecsx/pkg/aws/cloudwatch/types"
)

type metricsClient interface {
	GetMetricStatistics(context.Context, *cw.GetMetricStatisticsInput, ...func(*cw.Options)) (*cw.GetMetricStatisticsOutput, error)
}

// GetServiceMetrics retrieves CPU and memory utilization for an ECS service over the last hour.
func GetServiceMetrics(client metricsClient, ctx context.Context, cluster, service string) (*apitypes.ServiceMetrics, error) {
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

	m := &apitypes.ServiceMetrics{}
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

func getMetricStats(client metricsClient, ctx context.Context, metricName string, dims []cwtypes.Dimension, start, end time.Time) (*cw.GetMetricStatisticsOutput, error) {
	return client.GetMetricStatistics(ctx, &cw.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/ECS"),
		MetricName: aws.String(metricName),
		Dimensions: dims,
		StartTime:  &start,
		EndTime:    &end,
		Period:     aws.Int32(300),
		Statistics: []cwtypes.Statistic{cwtypes.StatisticAverage},
	})
}
