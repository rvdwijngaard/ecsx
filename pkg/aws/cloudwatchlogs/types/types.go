package types

import "time"

// LogEvent represents a single CloudWatch log event.
type LogEvent struct {
	Timestamp time.Time
	Message   string
	Stream    string
	EventID   string
	LogGroup  string
}

// ContainerLogGroup holds log configuration for a single container.
type ContainerLogGroup struct {
	Container    string
	LogGroup     string
	StreamPrefix string
}
