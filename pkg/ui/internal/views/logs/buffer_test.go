package logsview

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	adaptertypes "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
)

func makeLine(msg string) adaptertypes.FormattedLogLine {
	return adaptertypes.FormattedLogLine{
		Timestamp: time.Now(),
		Message:   msg,
		Raw:       msg,
	}
}

func TestRingBuffer_AppendAndLines(t *testing.T) {
	buf := NewRingBuffer(5)

	buf.Append(makeLine("a"), makeLine("b"), makeLine("c"))
	assert.Equal(t, 3, buf.Len())

	lines := buf.Lines()
	assert.Equal(t, "a", lines[0].Message)
	assert.Equal(t, "b", lines[1].Message)
	assert.Equal(t, "c", lines[2].Message)
}

func TestRingBuffer_Overflow(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Append(makeLine("a"), makeLine("b"), makeLine("c"))
	assert.Equal(t, 3, buf.Len())

	// Adding one more should evict "a"
	buf.Append(makeLine("d"))
	assert.Equal(t, 3, buf.Len())

	lines := buf.Lines()
	assert.Equal(t, "b", lines[0].Message)
	assert.Equal(t, "c", lines[1].Message)
	assert.Equal(t, "d", lines[2].Message)
}

func TestRingBuffer_OverflowMultiple(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Append(makeLine("a"), makeLine("b"), makeLine("c"))
	// Add 3 more, should fully replace
	buf.Append(makeLine("d"), makeLine("e"), makeLine("f"))
	assert.Equal(t, 3, buf.Len())

	lines := buf.Lines()
	assert.Equal(t, "d", lines[0].Message)
	assert.Equal(t, "e", lines[1].Message)
	assert.Equal(t, "f", lines[2].Message)
}

func TestRingBuffer_Clear(t *testing.T) {
	buf := NewRingBuffer(5)
	buf.Append(makeLine("a"), makeLine("b"))
	buf.Clear()

	assert.Equal(t, 0, buf.Len())
	assert.Nil(t, buf.Lines())
}

func TestRingBuffer_EmptyLines(t *testing.T) {
	buf := NewRingBuffer(5)
	assert.Nil(t, buf.Lines())
	assert.Equal(t, 0, buf.Len())
}

func TestRingBuffer_LargeOverflow(t *testing.T) {
	buf := NewRingBuffer(5)

	// Add 12 items to a buffer of size 5
	for i := 0; i < 12; i++ {
		buf.Append(makeLine(fmt.Sprintf("%d", i)))
	}

	assert.Equal(t, 5, buf.Len())
	lines := buf.Lines()
	// Should have the last 5: 7, 8, 9, 10, 11
	assert.Equal(t, "7", lines[0].Message)
	assert.Equal(t, "8", lines[1].Message)
	assert.Equal(t, "9", lines[2].Message)
	assert.Equal(t, "10", lines[3].Message)
	assert.Equal(t, "11", lines[4].Message)
}
