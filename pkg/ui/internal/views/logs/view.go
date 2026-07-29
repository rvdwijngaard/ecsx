package logsview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appconfig "github.com/rvdwijngaard/ecsx/pkg"
	cwladapter "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs"
	adaptertypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/util/keymaps"
)

var (
	borderStyle  = styles.BorderStyle
	focusedStyle = styles.FocusedBorderStyle
)

// LogsView displays CloudWatch container logs with history + live tail.
type LogsView struct {
	// shared config (clients accessed via config pointer)
	config *appconfig.Config

	// view state
	KeyMap    *LogsKeyMap
	AddKeyMap keymaps.AdditionalKeys

	// context for the current tail session
	tailCtx    context.Context
	tailCancel context.CancelFunc
	tailCh     <-chan adaptertypes.FormattedLogLine

	// external log formatter (e.g. "hl -L -F")
	// When configured, logs are opened in an external viewer via tea.ExecProcess
	// instead of the internal view. See external.go.

	// log buffer
	buffer *RingBuffer

	// viewport state (manual since we render from buffer)
	offset  int // line offset from top of buffer for display
	pinned  bool // true = user scrolled up, don't auto-scroll
	loading bool

	// spinner
	spinner spinner.Model

	// current log target
	cluster   string
	service   string
	container string

	// window dimensions
	window struct {
		width  int
		height int
	}

	// error
	err error
}

type Option func(*LogsView)

func WithAdditionalKeys(keys keymaps.AdditionalKeys) Option {
	return func(v *LogsView) {
		v.AddKeyMap = keys
	}
}

// NewLogsView creates a new logs view.
func NewLogsView(config *appconfig.Config, opts ...Option) *LogsView {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	v := &LogsView{
		config:  config,
		KeyMap:  DefaultLogsKeyMap(),
		buffer:  NewRingBuffer(defaultBufferCapacity),
		spinner: sp,
	}

	for _, o := range opts {
		o(v)
	}

	return v
}

// Init resets the logs view state.
func (m *LogsView) Init() tea.Cmd {
	m.stopTail()
	m.buffer.Clear()
	m.offset = 0
	m.pinned = false
	m.loading = false
	m.err = nil
	m.cluster = ""
	m.service = ""
	m.container = ""
	return nil
}

// Update handles messages for the logs view.
func (m *LogsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case messages.OpenLogs:
		if msg.Container == "" {
			return nil // wait for container to be resolved
		}
		return m.openLogs(msg)
	case messages.LogBatch:
		return m.handleLogBatch(msg)
	case messages.LogTailError:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		}
		return nil
	case messages.CloseLogs:
		m.stopTail()
		return nil
	case logTailStartedMsg:
		m.tailCh = msg.ch
		return m.nextTailBatch()
	case tea.WindowSizeMsg:
		m.ApplySize(msg.Height-1, msg.Width)
		return nil
	case spinner.TickMsg:
		if !m.loading {
			return nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return nil
}

func (m *LogsView) openLogs(msg messages.OpenLogs) tea.Cmd {
	m.stopTail()
	m.buffer.Clear()
	m.offset = 0
	m.pinned = false
	m.err = nil
	m.loading = true
	m.cluster = msg.Cluster
	m.service = msg.Service
	m.container = msg.Container

	return tea.Batch(m.spinner.Tick, m.fetchAndTail())
}

func (m *LogsView) fetchAndTail() tea.Cmd {
	config := m.config
	cluster := m.cluster
	service := m.service
	container := m.container

	return func() tea.Msg {
		ctx := context.Background()

		ecsClient := config.ECSClient
		if ecsClient == nil {
			return messages.LogTailError{Err: fmt.Errorf("ECS client not available")}
		}
		cwlClient := config.CloudWatchLogsClient
		if cwlClient == nil {
			return messages.LogTailError{Err: fmt.Errorf("CloudWatch Logs client not available")}
		}

		// Resolve log group
		logCfg, err := cwladapter.ResolveLogGroup(ecsClient, ctx, cluster, service, container)
		if err != nil {
			return messages.LogTailError{Err: fmt.Errorf("resolving log group: %w", err)}
		}

		// Fetch history
		history, err := cwladapter.FetchHistory(cwlClient, ctx, *logCfg)
		if err != nil {
			// Non-fatal: we can still tail even if history fetch fails
			return messages.LogBatch{Lines: nil}
		}

		return messages.LogBatch{Lines: history}
	}
}

func (m *LogsView) startTailCmd() tea.Cmd {
	config := m.config
	cluster := m.cluster
	service := m.service
	container := m.container

	// Create a cancellable context for the tail session
	ctx, cancel := context.WithCancel(context.Background())
	m.tailCtx = ctx
	m.tailCancel = cancel

	return func() tea.Msg {
		ecsClient := config.ECSClient
		cwlClient := config.CloudWatchLogsClient
		if ecsClient == nil || cwlClient == nil {
			return messages.LogTailError{Err: fmt.Errorf("clients not available")}
		}

		logCfg, err := cwladapter.ResolveLogGroup(ecsClient, ctx, cluster, service, container)
		if err != nil {
			return messages.LogTailError{Err: fmt.Errorf("resolving log group for tail: %w", err)}
		}

		ch, err := cwladapter.StartTail(cwlClient, ctx, *logCfg)
		if err != nil {
			return messages.LogTailError{Err: fmt.Errorf("starting tail: %w", err)}
		}

		// Store channel reference via a message so the view can chain reads
		return logTailStartedMsg{ch: ch}
	}
}

// logTailStartedMsg is an internal message carrying the tail channel.
type logTailStartedMsg struct {
	ch <-chan adaptertypes.FormattedLogLine
}

// nextTailBatch schedules reading the next batch from the stored tail channel.
func (m *LogsView) nextTailBatch() tea.Cmd {
	if m.tailCh == nil || m.tailCtx == nil {
		return nil
	}
	ch := m.tailCh
	ctx := m.tailCtx
	return waitForLogBatch(ch, ctx)
}

func (m *LogsView) handleLogBatch(msg messages.LogBatch) tea.Cmd {
	wasAtBottom := !m.pinned

	if len(msg.Lines) > 0 {
		m.buffer.Append(msg.Lines...)
	}

	// If this is the first batch (loading state), start the tail
	if m.loading {
		m.loading = false
		if wasAtBottom {
			m.scrollToBottom()
		}
		return m.startTailCmd()
	}

	// Auto-scroll if not pinned
	if wasAtBottom {
		m.scrollToBottom()
	}

	// Continue reading from the tail channel
	return m.nextTailBatch()
}

func (m *LogsView) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.KeyMap.Esc):
		m.stopTail()
		return func() tea.Msg {
			return messages.SwitchView{
				OldView: messages.Logs_view,
				NewView: messages.Task_selection,
			}
		}
	case key.Matches(msg, m.KeyMap.Up):
		m.pinned = true
		m.offset = max(0, m.offset-1)
	case key.Matches(msg, m.KeyMap.Down):
		m.offset = min(m.maxOffset(), m.offset+1)
		if m.offset >= m.maxOffset() {
			m.pinned = false
		}
	case key.Matches(msg, m.KeyMap.PageUp):
		m.pinned = true
		m.offset = max(0, m.offset-m.viewportHeight())
	case key.Matches(msg, m.KeyMap.PageDn):
		m.offset = min(m.maxOffset(), m.offset+m.viewportHeight())
		if m.offset >= m.maxOffset() {
			m.pinned = false
		}
	case key.Matches(msg, m.KeyMap.Top):
		m.pinned = true
		m.offset = 0
	case key.Matches(msg, m.KeyMap.Bottom):
		m.pinned = false
		m.scrollToBottom()
	default:
		if match, call := m.AddKeyMap.Matches(msg); match {
			return call
		}
	}
	return nil
}

func (m *LogsView) stopTail() {
	if m.tailCancel != nil {
		m.tailCancel()
		m.tailCancel = nil
		m.tailCtx = nil
	}
}

func (m *LogsView) scrollToBottom() {
	m.offset = m.maxOffset()
}

func (m *LogsView) maxOffset() int {
	total := m.buffer.Len()
	vh := m.viewportHeight()
	if total <= vh {
		return 0
	}
	return total - vh
}

func (m *LogsView) viewportHeight() int {
	// Account for border (2) + header line (1)
	h := m.window.height - 5
	if h < 1 {
		h = 1
	}
	return h
}

// ApplySize updates the view dimensions.
func (m *LogsView) ApplySize(height, width int) {
	m.window.height = height
	m.window.width = width
}

// View renders the logs view.
func (m *LogsView) View() string {
	w := m.window.width
	h := m.window.height

	if w == 0 || h == 0 {
		return ""
	}

	// Header
	header := m.renderHeader()

	// Content
	var content string
	if m.err != nil {
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(
			fmt.Sprintf("Error: %v", m.err),
		)
	} else if m.loading {
		content = fmt.Sprintf("%s Loading logs...", m.spinner.View())
	} else {
		content = m.renderLogLines()
	}

	// Status bar
	status := m.renderStatus()

	innerW := w - 4
	if innerW < 1 {
		innerW = 1
	}
	innerH := h - 4 // border + header + status
	if innerH < 1 {
		innerH = 1
	}

	body := lipgloss.Place(innerW, innerH, lipgloss.Left, lipgloss.Top, content)
	full := lipgloss.JoinVertical(lipgloss.Left, header, body, status)

	border := focusedStyle.
		Height(h - 2).
		Width(w - 2)

	return border.Render(full)
}

func (m *LogsView) renderHeader() string {
	title := fmt.Sprintf(" Logs: %s/%s", m.service, m.container)
	if m.container == "" {
		title = fmt.Sprintf(" Logs: %s", m.service)
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render(title)
}

func (m *LogsView) renderStatus() string {
	parts := []string{}
	if m.pinned {
		parts = append(parts, "PAUSED")
	} else {
		parts = append(parts, "LIVE")
	}
	parts = append(parts, fmt.Sprintf("%d lines", m.buffer.Len()))

	status := strings.Join(parts, " │ ")
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render(" " + status)
}

func (m *LogsView) renderLogLines() string {
	lines := m.buffer.Lines()
	if len(lines) == 0 {
		return "Waiting for log events..."
	}

	vh := m.viewportHeight()
	start := m.offset
	end := start + vh
	if end > len(lines) {
		end = len(lines)
	}
	if start >= len(lines) {
		start = max(0, len(lines)-vh)
		end = len(lines)
	}

	visible := lines[start:end]
	rendered := make([]string, len(visible))
	for i, line := range visible {
		rendered[i] = m.truncateLine(line.Raw)
	}
	return strings.Join(rendered, "\n")
}

func (m *LogsView) truncateLine(line string) string {
	maxW := m.window.width - 6
	if maxW <= 0 {
		return ""
	}
	runes := []rune(line)
	if len(runes) > maxW {
		return string(runes[:maxW-1]) + "…"
	}
	return line
}

// waitForLogBatch reads from the tail channel, batching events to avoid
// flooding the Bubble Tea event loop.
func waitForLogBatch(ch <-chan adaptertypes.FormattedLogLine, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		var batch []adaptertypes.FormattedLogLine
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()

		for {
			select {
			case line, ok := <-ch:
				if !ok {
					// Channel closed — stream ended
					if len(batch) > 0 {
						return messages.LogBatch{Lines: batch}
					}
					return messages.LogTailError{Err: nil}
				}
				batch = append(batch, line)
				if len(batch) >= 50 {
					return messages.LogBatch{Lines: batch}
				}
			case <-timer.C:
				if len(batch) > 0 {
					return messages.LogBatch{Lines: batch}
				}
				timer.Reset(100 * time.Millisecond)
			case <-ctx.Done():
				if len(batch) > 0 {
					return messages.LogBatch{Lines: batch}
				}
				return messages.CloseLogs{}
			}
		}
	}
}
