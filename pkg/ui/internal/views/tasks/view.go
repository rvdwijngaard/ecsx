package taskselection

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appconfig "github.com/ron/ecsx/pkg"
	"github.com/ron/ecsx/pkg/ui/internal/messages"
	"github.com/ron/ecsx/pkg/ui/internal/styles"
	"github.com/ron/ecsx/pkg/ui/internal/views/util/keymaps"
	u "github.com/ron/ecsx/pkg/util"
)

type paneID int

const (
	taskPaneID paneID = iota
	detailsPaneID
)

type TaskSelection struct {
	// shared config
	config *appconfig.Config

	// view window
	window struct {
		width  int
		height int
	}

	// key map
	KeyMap *TaskViewKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	// panes
	taskPane    *taskSelectionPane
	detailsPane *detailsPane

	zoomEnabled bool

	focused    paneID
	zoomtarget paneID
}

var (
	borderStyle  = styles.BorderStyle
	focusedStyle = styles.FocusedBorderStyle
)

func (m *TaskSelection) renderBorder(paneID paneID, content string) string {
	if m.focused == paneID {
		return focusedStyle.Render(content)
	}
	return borderStyle.Render(content)
}

type Option func(t *TaskSelection)

func WithAdditionalKeys(keys keymaps.AdditionalKeys) Option {
	return func(t *TaskSelection) {
		t.AddKeyMap = keys
	}
}

func NewTaskSelectionView(ctx context.Context, config *appconfig.Config, opts ...Option) *TaskSelection {
	t := &TaskSelection{
		config: config,
		KeyMap: DefaultTaskViewKeyMap(),
	}

	for _, o := range opts {
		o(t)
	}

	t.taskPane = newTaskSelectionPane(ctx, config, withTaskPaneKeys(t.AddKeyMap))
	t.detailsPane = newDetailsPane(withDetailsPaneKeys(t.AddKeyMap))

	return t
}

func (m *TaskSelection) Init() tea.Cmd {
	return tea.Batch(m.taskPane.Init(), m.detailsPane.Init())
}

// Update handles the message and if it does not detect a keypress that it can
// map itself proceeds to forward the message to the model's children.
func (m *TaskSelection) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.MoveFocus):
			m.moveFocus()
			return nil
		}
	case tea.WindowSizeMsg:
		m.window.height = msg.Height
		m.window.width = msg.Width
		m.applySize()
	case messages.ZoomToggleTaskSelectionPane, messages.ZoomToggleTaskDetailsPane:
		cmd = m.handleZoom(msg)
	}

	return tea.Batch(cmd, m.forward(msg))
}

// forward takes a message and decides to broadcast or to forward only to focused children.
func (m *TaskSelection) forward(msg tea.Msg) tea.Cmd {
	if _, isKeyPress := msg.(tea.KeyPressMsg); isKeyPress {
		return m.routeToFocusedOnly(msg)
	}
	return m.broadcast(msg)
}

// broadcast takes a message and forwards it to all children.
func (m TaskSelection) broadcast(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	cmds = append(cmds, m.taskPane.Update(msg))
	cmds = append(cmds, m.detailsPane.Update(msg))
	return tea.Batch(cmds...)
}

// routeToFocusedOnly takes a message and only routes it to the focused child.
func (m *TaskSelection) routeToFocusedOnly(msg tea.Msg) tea.Cmd {
	switch m.focused {
	case taskPaneID:
		return m.taskPane.Update(msg)
	case detailsPaneID:
		return m.detailsPane.Update(msg)
	default:
		panic("focused pane not found")
	}
}

func (m *TaskSelection) handleZoom(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case messages.ZoomToggleTaskSelectionPane:
		m.zoomEnabled = !m.zoomEnabled
		m.zoomtarget = taskPaneID
		m.focused = taskPaneID
		m.KeyMap.MoveFocus.SetEnabled(!m.KeyMap.MoveFocus.Enabled())
	case messages.ZoomToggleTaskDetailsPane:
		m.zoomEnabled = !m.zoomEnabled
		m.zoomtarget = detailsPaneID
		m.focused = detailsPaneID
		m.KeyMap.MoveFocus.SetEnabled(!m.KeyMap.MoveFocus.Enabled())
	}
	m.applySize()
	return nil
}

func (m *TaskSelection) applySize() {
	w := u.Ternary(m.window.width, m.window.width/2, m.zoomEnabled)
	borderStyle = borderStyle.
		Height(m.window.height - 2).
		Width(w)

	focusedStyle = focusedStyle.
		Height(m.window.height - 2).
		Width(w)

	m.taskPane.applySize(m.window.height-2-3, w-4)
	m.detailsPane.applySize(m.window.height-2-3, w-4)
}

func (m *TaskSelection) moveFocus() {
	m.focused++
	if m.focused > detailsPaneID {
		m.focused = taskPaneID
	}
}

func (m *TaskSelection) View() string {
	s := strings.Builder{}
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		u.Ternary(m.renderBorder(taskPaneID, m.taskPane.View()), "", !m.zoomEnabled || m.zoomtarget == taskPaneID),
		u.Ternary(m.renderBorder(detailsPaneID, m.detailsPane.View()), "", !m.zoomEnabled || m.zoomtarget == detailsPaneID),
	))
	return s.String()
}
