package serviceselection

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
	servicePaneID paneID = iota
	detailsPaneID
	metricsPaneID
)

type ServiceSelection struct {
	// shared config
	config *appconfig.Config

	// view window
	window struct {
		width  int
		height int
	}

	// key map
	KeyMap *ServiceViewKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	// panes
	servicePane *serviceSelectionPane
	detailsPane *detailsPane
	metricsPane *metricsPane

	zoomEnabled bool

	focused    paneID
	zoomtarget paneID
}

var (
	borderStyle  = styles.BorderStyle
	focusedStyle = styles.FocusedBorderStyle
)

func (m *ServiceSelection) renderBorder(paneID paneID, content string) string {
	if m.focused == paneID {
		return focusedStyle.Render(content)
	}
	return borderStyle.Render(content)
}

type Option func(t *ServiceSelection)

func WithAdditionalKeys(keys keymaps.AdditionalKeys) Option {
	return func(t *ServiceSelection) {
		t.AddKeyMap = keys
	}
}

func NewServiceSelectionView(ctx context.Context, config *appconfig.Config, opts ...Option) *ServiceSelection {
	t := &ServiceSelection{
		config: config,
		KeyMap: DefaultServiceViewKeyMap(),
	}

	for _, o := range opts {
		o(t)
	}

	t.servicePane = newServiceSelectionPane(ctx, config, withServicePaneKeys(t.AddKeyMap))
	t.detailsPane = newDetailsPane(withDetailsPaneKeys(t.AddKeyMap))
	t.metricsPane = newMetricsPane(config)

	return t
}

func (m *ServiceSelection) Init() tea.Cmd {
	return tea.Batch(m.servicePane.Init(), m.detailsPane.Init(), m.metricsPane.Init())
}

// Update handles the message and if it does not detect a keypress that it can
// map itself proceeds to forward the message to the model's children.
func (m *ServiceSelection) Update(msg tea.Msg) tea.Cmd {
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
	case messages.ZoomToggleServiceSelectionPane, messages.ZoomToggleServiceDetailsPane:
		cmd = m.handleZoom(msg)
	}

	return tea.Batch(cmd, m.forward(msg))
}

// forward takes a message and decides to broadcast or to forward only to focused children.
func (m *ServiceSelection) forward(msg tea.Msg) tea.Cmd {
	if _, isKeyPress := msg.(tea.KeyPressMsg); isKeyPress {
		return m.routeToFocusedOnly(msg)
	}
	return m.broadcast(msg)
}

// broadcast takes a message and forwards it to all children.
func (m ServiceSelection) broadcast(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	cmds = append(cmds, m.servicePane.Update(msg))
	cmds = append(cmds, m.detailsPane.Update(msg))
	cmds = append(cmds, m.metricsPane.Update(msg))
	return tea.Batch(cmds...)
}

// routeToFocusedOnly takes a message and only routes it to the focused child.
func (m *ServiceSelection) routeToFocusedOnly(msg tea.Msg) tea.Cmd {
	switch m.focused {
	case servicePaneID:
		return m.servicePane.Update(msg)
	case detailsPaneID:
		return m.detailsPane.Update(msg)
	default:
		panic("focused pane not found")
	}
}

func (m *ServiceSelection) handleZoom(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case messages.ZoomToggleServiceSelectionPane:
		m.zoomEnabled = !m.zoomEnabled
		m.zoomtarget = servicePaneID
		m.focused = servicePaneID
		m.KeyMap.MoveFocus.SetEnabled(!m.KeyMap.MoveFocus.Enabled())
	case messages.ZoomToggleServiceDetailsPane:
		m.zoomEnabled = !m.zoomEnabled
		m.zoomtarget = detailsPaneID
		m.focused = detailsPaneID
		m.KeyMap.MoveFocus.SetEnabled(!m.KeyMap.MoveFocus.Enabled())
	}
	m.applySize()
	return nil
}

func (m *ServiceSelection) applySize() {
	w := u.Ternary(m.window.width, m.window.width/2, m.zoomEnabled)

	// Right side splits between details (top) and metrics (bottom)
	rightH := m.window.height - 2
	metricsH := min(20, rightH/2) // metrics chart gets up to half of right pane
	detailsH := rightH - metricsH

	borderStyle = borderStyle.
		Height(m.window.height - 2).
		Width(w)

	focusedStyle = focusedStyle.
		Height(m.window.height - 2).
		Width(w)

	m.servicePane.applySize(m.window.height-2-3, w-4)
	m.detailsPane.applySize(detailsH-3, w-4)
	m.metricsPane.applySize(metricsH-1, w-4)
}

func (m *ServiceSelection) moveFocus() {
	m.focused++
	if m.focused > detailsPaneID {
		m.focused = servicePaneID
	}
}

func (m *ServiceSelection) View() string {
	// Right pane: details + metrics stacked vertically
	rightContent := lipgloss.JoinVertical(lipgloss.Left,
		m.detailsPane.View(),
		m.metricsPane.View(),
	)

	s := strings.Builder{}
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		u.Ternary(m.renderBorder(servicePaneID, m.servicePane.View()), "", !m.zoomEnabled || m.zoomtarget == servicePaneID),
		u.Ternary(m.renderBorder(detailsPaneID, rightContent), "", !m.zoomEnabled || m.zoomtarget == detailsPaneID),
	))
	return s.String()
}
