package logsview

import (
	adaptertypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
)

const defaultBufferCapacity = 5000

// RingBuffer is a fixed-capacity circular buffer for log lines.
// When full, new entries overwrite the oldest.
type RingBuffer struct {
	lines    []adaptertypes.FormattedLogLine
	capacity int
	head     int // index of the oldest element
	size     int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = defaultBufferCapacity
	}
	return &RingBuffer{
		lines:    make([]adaptertypes.FormattedLogLine, capacity),
		capacity: capacity,
	}
}

// Append adds one or more lines to the buffer. If the buffer is full,
// the oldest lines are overwritten.
func (b *RingBuffer) Append(lines ...adaptertypes.FormattedLogLine) {
	for _, line := range lines {
		idx := (b.head + b.size) % b.capacity
		if b.size == b.capacity {
			// overwrite oldest, advance head
			b.lines[b.head] = line
			b.head = (b.head + 1) % b.capacity
		} else {
			b.lines[idx] = line
			b.size++
		}
	}
}

// Lines returns all buffered lines in order from oldest to newest.
func (b *RingBuffer) Lines() []adaptertypes.FormattedLogLine {
	if b.size == 0 {
		return nil
	}
	result := make([]adaptertypes.FormattedLogLine, b.size)
	for i := 0; i < b.size; i++ {
		result[i] = b.lines[(b.head+i)%b.capacity]
	}
	return result
}

// Len returns the number of lines currently in the buffer.
func (b *RingBuffer) Len() int {
	return b.size
}

// Clear resets the buffer.
func (b *RingBuffer) Clear() {
	b.head = 0
	b.size = 0
}
