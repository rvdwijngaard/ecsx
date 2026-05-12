package types

import "time"

// LogConfig holds the configuration needed to fetch and tail logs.
type LogConfig struct {
	LogGroup         string
	StreamPrefix     string
	Container        string
	LookbackDuration time.Duration
}

// FormattedLogLine represents a single log line ready for display.
type FormattedLogLine struct {
	Timestamp time.Time
	Stream    string
	Message   string
	Raw       string // pre-rendered line for viewport
}
