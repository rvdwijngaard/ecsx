package tasks

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/ron/ecsx/pkg/ui/internal/styles"
	u "github.com/ron/ecsx/pkg/util"
)

type paneID int

const (
	listPaneID paneID = iota
	detailsPaneID
)

// View is the top-level tasks view with list + details panes.
type View struct {
	// view window
	window struct {
		width  int
		height int
	}

	// key map
	KeyMap *ViewKeyMap

	// panes
	listPane    *listPane
	detailsPane *detailsPane

	zoomEnabled bool
	focused     paneID
	zoomTarget  paneID
}

func NewView(ctx context.Context, client *ecs.Client) *View {
	v := &View{
		KeyMap:      DefaultViewKeyMap(),
		listPane:    newListPane(ctx, client),
		detailsPane: newDetailsPane(),
	}
	return v
}

// Load starts loading tasks for the given cluster and service.
func (m *View) Load(cluster, service string) tea.Cmd {
	return m.listPane.Load(cluster, service)
}

func (m *View) Update(msg tea.Msg) tea.Cmd {
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
	case zoomToggleListPane:
		cmd = m.handleZoom(listPaneID)
	case zoomToggleDetailsPane:
		cmd = m.handleZoom(detailsPaneID)
	}

	return tea.Batch(cmd, m.forward(msg))
}

func (m *View) forward(msg tea.Msg) tea.Cmd {
	if _, isKeyPress := msg.(tea.KeyPressMsg); isKeyPress {
		return m.routeToFocusedOnly(msg)
	}
	return m.broadcast(msg)
}

func (m *View) broadcast(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	cmds = append(cmds, m.listPane.Update(msg))
	cmds = append(cmds, m.detailsPane.Update(msg))
	return tea.Batch(cmds...)
}

func (m *View) routeToFocusedOnly(msg tea.Msg) tea.Cmd {
	switch m.focused {
	case listPaneID:
		return m.listPane.Update(msg)
	case detailsPaneID:
		return m.detailsPane.Update(msg)
	default:
		return nil
	}
}

func (m *View) handleZoom(target paneID) tea.Cmd {
	m.zoomEnabled = !m.zoomEnabled
	m.zoomTarget = target
	m.focused = target
	m.KeyMap.MoveFocus.SetEnabled(!m.zoomEnabled)
	m.applySize()
	return nil
}

func (m *View) applySize() {
	w := u.Ternary(m.window.width, m.window.width/2, m.zoomEnabled)
	m.listPane.applySize(m.window.height-2-3, w-4)
	m.detailsPane.applySize(m.window.height-2-3, w-4)
}

func (m *View) moveFocus() {
	m.focused++
	if m.focused > detailsPaneID {
		m.focused = listPaneID
	}
}

func (m *View) renderBorder(pane paneID, content string) string {
	w := u.Ternary(m.window.width, m.window.width/2, m.zoomEnabled)

	bs := styles.BorderStyle.
		Height(m.window.height - 2).
		Width(w)

	fs := styles.FocusedBorderStyle.
		Height(m.window.height - 2).
		Width(w)

	if m.focused == pane {
		return fs.Render(content)
	}
	return bs.Render(content)
}

func (m *View) View() string {
	s := strings.Builder{}
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		u.Ternary(m.renderBorder(listPaneID, m.listPane.View()), "", !m.zoomEnabled || m.zoomTarget == listPaneID),
		u.Ternary(m.renderBorder(detailsPaneID, m.detailsPane.View()), "", !m.zoomEnabled || m.zoomTarget == detailsPaneID),
	))
	return s.String()
}

// BackToServicesMsg signals navigation back to services view.
type BackToServicesMsg struct {
	Cluster string
}
