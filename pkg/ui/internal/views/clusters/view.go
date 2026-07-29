package clusterselection

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appconfig "github.com/rvdwijngaard/ecsx/pkg"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/util/keymaps"
	u "github.com/rvdwijngaard/ecsx/pkg/util"
)

type paneID int

const (
	clusterPaneID paneID = iota
	detailsPaneID
)

type ClusterSelection struct {
	// view window
	window struct {
		width  int
		height int
	}

	// key map
	KeyMap *ClusterViewKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	// panes
	clusterPane *clusterSelectionPane
	detailsPane *detailsPane

	zoomEnabled bool

	focused    paneID
	zoomtarget paneID
}

var (
	borderStyle  = styles.BorderStyle
	focusedStyle = styles.FocusedBorderStyle
)

func (m *ClusterSelection) renderBorder(paneID paneID, content string) string {
	if m.focused == paneID {
		return focusedStyle.Render(content)
	}
	return borderStyle.Render(content)
}

type Option func(t *ClusterSelection)

func WithAdditionalKeys(keys keymaps.AdditionalKeys) Option {
	return func(t *ClusterSelection) {
		t.AddKeyMap = keys
	}
}

func NewClusterSelectionView(ctx context.Context, config *appconfig.Config, opts ...Option) *ClusterSelection {
	t := &ClusterSelection{
		KeyMap: DefaultClusterViewKeyMap(),
	}

	for _, o := range opts {
		o(t)
	}

	t.clusterPane = newClusterSelectionPane(ctx, config, withClusterPaneKeys(t.AddKeyMap))
	t.detailsPane = newDetailsPane(withDetailsPaneKeys(t.AddKeyMap))

	return t
}

func (m *ClusterSelection) Init() tea.Cmd {
	return tea.Batch(m.clusterPane.Init(), m.detailsPane.Init())
}

// Update handles the message and if it does not detect a keypress that it can
// map itself proceeds to forward the message to the model's children.
func (m *ClusterSelection) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.MoveFocus):
			m.moveFocus()
			return nil
		case key.Matches(msg, m.KeyMap.Regions):
			return m.ToggleRegionsDialog()
		}
	case tea.WindowSizeMsg:
		m.window.height = msg.Height
		m.window.width = msg.Width
		m.applySize()
	case messages.ZoomToggleClusterSelectionPane, messages.ZoomToggleClusterDetailsPane:
		cmd = m.handleZoom(msg)
	}

	return tea.Batch(cmd, m.forward(msg))
}

// forward takes a message and decides to broadcast or to forward only to focused children.
func (m *ClusterSelection) forward(msg tea.Msg) tea.Cmd {
	if _, isKeyPress := msg.(tea.KeyPressMsg); isKeyPress {
		return m.routeToFocusedOnly(msg)
	}
	return m.broadcast(msg)
}

// broadcast takes a message and forwards it to all children.
func (m ClusterSelection) broadcast(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	cmds = append(cmds, m.clusterPane.Update(msg))
	cmds = append(cmds, m.detailsPane.Update(msg))
	return tea.Batch(cmds...)
}

// routeToFocusedOnly takes a message and only routes it to the focused child.
func (m *ClusterSelection) routeToFocusedOnly(msg tea.Msg) tea.Cmd {
	switch m.focused {
	case clusterPaneID:
		return m.clusterPane.Update(msg)
	case detailsPaneID:
		return m.detailsPane.Update(msg)
	default:
		panic("focused pane not found")
	}
}

func (m *ClusterSelection) handleZoom(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case messages.ZoomToggleClusterSelectionPane:
		m.zoomEnabled = !m.zoomEnabled
		m.zoomtarget = clusterPaneID
		m.focused = clusterPaneID
		m.KeyMap.MoveFocus.SetEnabled(!m.KeyMap.MoveFocus.Enabled())
	case messages.ZoomToggleClusterDetailsPane:
		m.zoomEnabled = !m.zoomEnabled
		m.zoomtarget = detailsPaneID
		m.focused = detailsPaneID
		m.KeyMap.MoveFocus.SetEnabled(!m.KeyMap.MoveFocus.Enabled())
	}
	m.applySize()
	return nil
}

func (m ClusterSelection) ToggleRegionsDialog() tea.Cmd {
	return func() tea.Msg {
		return messages.ToggleRegions{}
	}
}

func (m *ClusterSelection) applySize() {
	w := u.Ternary(m.window.width, m.window.width/2, m.zoomEnabled)
	borderStyle = borderStyle.
		Height(m.window.height - 2).
		Width(w)

	focusedStyle = focusedStyle.
		Height(m.window.height - 2).
		Width(w)

	m.clusterPane.applySize(m.window.height-2-3, w-4)
	m.detailsPane.applySize(m.window.height-2-3, w-4)
}

func (m *ClusterSelection) moveFocus() {
	m.focused++
	if m.focused > detailsPaneID {
		m.focused = clusterPaneID
	}
}

func (m *ClusterSelection) View() string {
	s := strings.Builder{}
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		u.Ternary(m.renderBorder(clusterPaneID, m.clusterPane.View()), "", !m.zoomEnabled || m.zoomtarget == clusterPaneID),
		u.Ternary(m.renderBorder(detailsPaneID, m.detailsPane.View()), "", !m.zoomEnabled || m.zoomtarget == detailsPaneID),
	))
	return s.String()
}
