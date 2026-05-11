package clusters

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	apitypes "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs/types"
	"github.com/ron/ecsx/pkg/ui/internal/styles"
)

type detailsPane struct {
	// pane's view window
	window struct {
		width  int
		height int
	}

	// key map
	KeyMap *DetailsPaneKeyMap

	// styles
	dStyles detailsStyles

	content viewport.Model
}

type detailsStyles struct {
	headerStyle    lipgloss.Style
	fieldNameStyle lipgloss.Style
}

func newDetailsPane() *detailsPane {
	c := viewport.New(viewport.WithHeight(20))
	c.SoftWrap = false
	c.SetHorizontalStep(5)

	p := &detailsPane{
		content: c,
		KeyMap:  DefaultDetailsPaneKeyMap(),
	}

	p.dStyles = detailsStyles{
		headerStyle:    lipgloss.NewStyle().Bold(true).Foreground(styles.ViewFocusBorderColour).PaddingBottom(1),
		fieldNameStyle: lipgloss.NewStyle().Foreground(styles.SubtleColour),
	}

	return p
}

func (m *detailsPane) Init() tea.Cmd {
	return nil
}

func (m *detailsPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Zoom):
			return m.zoom()
		}
	case clusterDetailsMsg:
		m.content.SetContent(renderClusterDetails(msg.cluster, m.dStyles))
		return nil
	}

	var cmd tea.Cmd
	m.content, cmd = m.content.Update(msg)
	return cmd
}

func (m *detailsPane) zoom() tea.Cmd {
	return func() tea.Msg {
		return zoomToggleDetailsPane{}
	}
}

func (m *detailsPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.content.SetHeight(height)
	m.content.SetWidth(width)
}

func (m *detailsPane) View() string {
	return m.content.View()
}

func renderClusterDetails(cluster *apitypes.ClusterItem, st detailsStyles) string {
	if cluster == nil {
		return ""
	}

	header := st.headerStyle.Render
	field := st.fieldNameStyle.Render

	s := strings.Builder{}
	fmt.Fprintf(&s, "%s\n", header("CLUSTER"))
	fmt.Fprintf(&s, "%s:  %s\n", field("Name"), cluster.Name)
	fmt.Fprintf(&s, "%s:   %s\n", field("ARN"), cluster.ARN)
	fmt.Fprintf(&s, "%s:%s\n", field("Status"), statusWithColor(cluster.Status))
	fmt.Fprintf(&s, "\n")
	fmt.Fprintf(&s, "%s\n", header("RESOURCES"))
	fmt.Fprintf(&s, "%s:  %d\n", field("Active Services"), cluster.ActiveServices)
	fmt.Fprintf(&s, "%s:   %d\n", field("Running Tasks"), cluster.RunningTasks)
	fmt.Fprintf(&s, "%s:   %d\n", field("Pending Tasks"), cluster.PendingTasks)
	fmt.Fprintf(&s, "%s:  %d\n", field("Container Instances"), cluster.ContainerInstances)

	return s.String()
}

func statusWithColor(status string) string {
	switch strings.ToUpper(status) {
	case "ACTIVE":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")).Render(" " + status)
	case "INACTIVE", "FAILED":
		return lipgloss.NewStyle().Foreground(styles.ErrorColour).Render(" " + status)
	default:
		return " " + status
	}
}

// Messages
type zoomToggleDetailsPane struct{}
