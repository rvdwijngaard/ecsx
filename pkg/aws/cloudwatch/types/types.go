package types

// ServiceMetrics holds time-ordered CPU and memory utilization data.
type ServiceMetrics struct {
	CPU     []float64 // time-ordered avg values
	Mem     []float64
	HasData bool
}
