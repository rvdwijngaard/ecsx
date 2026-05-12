package types

import "time"

// MetricPoint represents a single time-value pair for charting.
type MetricPoint struct {
	Time  time.Time
	Value float64
}

// MetricSeries holds min/max/avg data points for a single metric.
type MetricSeries struct {
	Min []MetricPoint
	Max []MetricPoint
	Avg []MetricPoint
}

// ServiceMetrics holds CPU and memory utilization data for an ECS service.
type ServiceMetrics struct {
	CPU     MetricSeries
	Mem     MetricSeries
	HasData bool
}
