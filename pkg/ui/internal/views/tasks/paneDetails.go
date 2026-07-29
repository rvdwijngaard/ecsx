package taskselection

import (
	"fmt"
	"strings"
	"time"

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
	case messages.TaskDetails:
		m.content.SetContent(renderTaskDetails(msg.Details, m.styles))
		return nil
	}

	m.content, cmd = m.content.Update(msg)
	return
}

func renderTaskDetails(details *apitypes.TaskItem, s detailsStyles) string {
	if details == nil {
		return ""
	}

	field := s.fieldNameStyle.Render
	header := s.headerStyle.Render

	b := strings.Builder{}
	fmt.Fprintf(&b, "  %s %s\n", field("Task ID"), details.ID)
	fmt.Fprintf(&b, "  %s %s\n", field("Status"), details.Status)
	fmt.Fprintf(&b, "  %s %s\n", field("Desired Status"), details.DesiredStatus)
	fmt.Fprintf(&b, "  %s %s\n", field("Health Status"), details.HealthStatus)
	fmt.Fprintf(&b, "  %s %s\n", field("Launch Type"), details.LaunchType)
	fmt.Fprintf(&b, "  %s %s\n", field("Task Definition"), details.TaskDefinition)
	fmt.Fprintf(&b, "  %s %s\n", field("Group"), details.Group)
	fmt.Fprintf(&b, "  %s %s / %s\n", field("CPU / Memory"), details.CPU, details.Memory)
	if details.StartedAt != nil {
		fmt.Fprintf(&b, "  %s %s\n", field("Started At"), details.StartedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "  %s %s\n", field("Uptime"), formatUptime(*details.StartedAt))
	}
	if details.CreatedAt != nil {
		fmt.Fprintf(&b, "  %s %s\n", field("Created At"), details.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if details.StoppedAt != nil {
		fmt.Fprintf(&b, "  %s %s\n", field("Stopped At"), details.StoppedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if details.StoppedReason != "" {
		fmt.Fprintf(&b, "  %s %s\n", field("Stopped Reason"), details.StoppedReason)
	}
	if details.EC2InstanceID != "" {
		fmt.Fprintf(&b, "  %s %s\n", field("EC2 Instance"), details.EC2InstanceID)
	}
	if details.PrivateIP != "" {
		fmt.Fprintf(&b, "  %s %s\n", field("Private IP"), details.PrivateIP)
	}
	if details.PublicIP != "" {
		fmt.Fprintf(&b, "  %s %s\n", field("Public IP"), details.PublicIP)
	}

	// Console URLs
	region := details.Region
	if region != "" {
		fmt.Fprintf(&b, "\n")
		if details.EC2InstanceID != "" {
			fmt.Fprintf(&b, "  EC2 Console:\n")
			fmt.Fprintf(&b, "  https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s\n", region, region, details.EC2InstanceID)
			fmt.Fprintf(&b, "\n")
		}
		if details.ClusterName != "" {
			fmt.Fprintf(&b, "  Task Console:\n")
			fmt.Fprintf(&b, "  https://%s.console.aws.amazon.com/ecs/home?region=%s#/clusters/%s/tasks/%s\n", region, region, details.ClusterName, details.ID)
			fmt.Fprintf(&b, "\n")
		}
	}

	// Containers
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, " %s\n\n", header("Containers"))
	for _, c := range details.Containers {
		fmt.Fprintf(&b, "  %s\n", c.Name)
		fmt.Fprintf(&b, "    Status: %s\n", c.Status)
		if c.HealthStatus != "" && c.HealthStatus != "UNKNOWN" {
			fmt.Fprintf(&b, "    Health: %s\n", c.HealthStatus)
		}
		fmt.Fprintf(&b, "    Image: %s\n", c.Image)
		if c.ExitCode != nil {
			fmt.Fprintf(&b, "    Exit Code: %d\n", *c.ExitCode)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func formatUptime(started time.Time) string {
	d := time.Since(started)
	if d < 0 {
		d = 0
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (m *detailsPane) Zoom() tea.Cmd {
	return func() tea.Msg {
		return messages.ZoomToggleTaskDetailsPane{}
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
