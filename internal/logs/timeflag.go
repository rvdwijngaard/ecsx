package logs

import (
	"fmt"
	"time"
)

// TimeFlag is a pflag.Value that accepts either a Go duration (2h30m, 45m)
// or a datetime (2024-01-15T09:00, 2024-01-15T09:00:00, 2024-01-15).
// Durations are interpreted as that amount of time before now.
type TimeFlag struct {
	raw string
	t   time.Time
}

var dateFormats = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

func (f *TimeFlag) String() string { return f.raw }
func (f *TimeFlag) Type() string   { return "time" }

func (f *TimeFlag) Set(s string) error {
	f.raw = s
	if d, err := time.ParseDuration(s); err == nil {
		f.t = time.Now().Add(-d)
		return nil
	}
	for _, fmt := range dateFormats {
		if t, err := time.Parse(fmt, s); err == nil {
			f.t = t
			return nil
		}
	}
	return fmt.Errorf("invalid time: %q (use duration like 2h30m or datetime like 2024-01-15T09:00)", s)
}

// Time returns the resolved absolute time. Zero value if not set.
func (f *TimeFlag) Time() time.Time { return f.t }

// TimePtr returns a pointer to the resolved time, or nil if not set.
func (f *TimeFlag) TimePtr() *time.Time {
	if f.raw == "" {
		return nil
	}
	t := f.t
	return &t
}

// Default sets a default duration-ago value.
func (f *TimeFlag) Default(d time.Duration) {
	f.t = time.Now().Add(-d)
	f.raw = d.String()
}
