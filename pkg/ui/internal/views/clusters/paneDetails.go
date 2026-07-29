package clusterselection

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	apitypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs/types"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/util/keymaps"
)

type detailsPane struct {
	// errorText
	err error

	// pane's view window
	window struct {
		width  int
		height int
	}

	// key map
	KeyMap *DetailsPaneKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	styles detailsStyles

	content viewport.Model
}

type detailsStyles struct {
	headerStyle    lipgloss.Style
	fieldNameStyle lipgloss.Style
}

type detailsPaneOption func(p *detailsPane)

func withDetailsPaneKeys(keys keymaps.AdditionalKeys) detailsPaneOption {
	return func(t *detailsPane) {
		t.AddKeyMap = keys
	}
}

func newDetailsPane(opts ...detailsPaneOption) *detailsPane {
	step := 5
	c := viewport.New(viewport.WithHeight(20))
	c.SoftWrap = false
	c.SetHorizontalStep(step)
	c.KeyMap.Left.SetHelp("←/h", "left")
	c.KeyMap.Right.SetHelp("→/l", "right")
	p := &detailsPane{
		content: c,
		KeyMap:  DefaultDetailsKeyMap(),
	}

	p.styles = detailsStyles{
		headerStyle:    lipgloss.NewStyle().Bold(true).Foreground(styles.ViewFocusBorderColour).PaddingBottom(1),
		fieldNameStyle: lipgloss.NewStyle().Foreground(styles.SubtleColour),
	}

	for _, o := range opts {
		o(p)
	}

	if !keymaps.UniqueKeyMaps(p.KeyMap.ShortHelp(), p.AddKeyMap.Bindings()) {
		panic("overlapping keymaps!")
	}
	return p
}

func (m *detailsPane) cleanSlate() {
	m.err = nil
}

func (m *detailsPane) Init() tea.Cmd {
	m.cleanSlate()
	return nil
}

func (m *detailsPane) Update(msg tea.Msg) (cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Zoom):
			return m.Zoom()
		default:
			if match, call := m.AddKeyMap.Matches(msg); match {
				return call
			}
		}
	case messages.ClusterDetails:
		m.content.SetContent(renderClusterDetails(msg.Details, m.styles))
		return nil
	}

	m.content, cmd = m.content.Update(msg)
	return
}

func renderClusterDetails(details *apitypes.ClusterItem, s detailsStyles) string {
	if details == nil {
		return ""
	}

	header := s.headerStyle.Render
	field := s.fieldNameStyle.Render

	b := strings.Builder{}
	fmt.Fprintf(&b, "%s\n", header("CLUSTER"))
	fmt.Fprintf(&b, "%s:  %s\n", field("Name"), details.Name)
	fmt.Fprintf(&b, "%s:   %s\n", field("ARN"), details.ARN)
	fmt.Fprintf(&b, "%s:%s\n", field("Status"), fmt.Sprintf("  %s", details.Status))
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "%s\n", header("RESOURCES"))
	fmt.Fprintf(&b, "%s:  %d\n", field("Container Instances"), details.ContainerInstances)
	fmt.Fprintf(&b, "%s:  %d\n", field("Active Services"), details.ActiveServices)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "%s\n", header("TASKS"))
	fmt.Fprintf(&b, "%s:  %d\n", field("Running Tasks"), details.RunningTasks)
	fmt.Fprintf(&b, "%s:  %d\n", field("Pending Tasks"), details.PendingTasks)
	return b.String()
}

func (m *detailsPane) Zoom() tea.Cmd {
	return func() tea.Msg {
		return messages.ZoomToggleTableDetailsPane{}
	}
}

func (m *detailsPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.content.SetHeight(height)
	m.content.SetWidth(width)
}

func (m *detailsPane) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	return m.content.View()
}
