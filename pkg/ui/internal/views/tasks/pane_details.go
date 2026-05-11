package tasks

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
	case taskDetailsMsg:
		m.content.SetContent(renderTaskDetails(msg.task, m.dStyles))
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

func renderTaskDetails(task *apitypes.TaskItem, st detailsStyles) string {
	if task == nil {
		return ""
	}

	header := st.headerStyle.Render
	field := st.fieldNameStyle.Render

	s := strings.Builder{}
	fmt.Fprintf(&s, "%s\n", header("TASK"))
	fmt.Fprintf(&s, "%s:  %s\n", field("ID"), task.ID)
	fmt.Fprintf(&s, "%s: %s\n", field("ARN"), task.ARN)
	fmt.Fprintf(&s, "%s:%s\n", field("Status"), statusWithColor(task.Status))
	fmt.Fprintf(&s, "%s:  %s\n", field("Desired Status"), task.DesiredStatus)
	fmt.Fprintf(&s, "%s:  %s\n", field("Health"), task.HealthStatus)
	fmt.Fprintf(&s, "\n")
	fmt.Fprintf(&s, "%s\n", header("CONFIGURATION"))
	fmt.Fprintf(&s, "%s:  %s\n", field("Task Definition"), task.TaskDefinition)
	fmt.Fprintf(&s, "%s:  %s\n", field("Launch Type"), task.LaunchType)
	if task.CPU != "" {
		fmt.Fprintf(&s, "%s:  %s / %s\n", field("CPU / Memory"), task.CPU, task.Memory)
	}
	if task.StartedAt != nil {
		fmt.Fprintf(&s, "%s:  %s\n", field("Started At"), task.StartedAt.Local().Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintf(&s, "\n")

	if len(task.Containers) > 0 {
		fmt.Fprintf(&s, "%s\n", header("CONTAINERS"))
		for _, c := range task.Containers {
			fmt.Fprintf(&s, "%s:  %s\n", field("Name"), c.Name)
			fmt.Fprintf(&s, "%s:%s\n", field("  Status"), statusWithColor(c.Status))
			fmt.Fprintf(&s, "%s:  %s\n", field("  Image"), c.Image)
			if c.HealthStatus != "" && c.HealthStatus != "UNKNOWN" {
				fmt.Fprintf(&s, "%s:  %s\n", field("  Health"), c.HealthStatus)
			}
			if c.ExitCode != nil {
				fmt.Fprintf(&s, "%s:  %d\n", field("  Exit Code"), *c.ExitCode)
			}
			fmt.Fprintf(&s, "\n")
		}
	}

	return s.String()
}

func statusWithColor(status string) string {
	switch strings.ToUpper(status) {
	case "RUNNING", "ACTIVE":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")).Render(" " + status)
	case "STOPPED", "INACTIVE", "FAILED":
		return lipgloss.NewStyle().Foreground(styles.ErrorColour).Render(" " + status)
	case "PENDING", "PROVISIONING":
		return lipgloss.NewStyle().Foreground(styles.NumberColour).Render(" " + status)
	default:
		return " " + status
	}
}

// Messages
type zoomToggleDetailsPane struct{}
