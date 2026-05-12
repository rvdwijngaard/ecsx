package envvars

import (
	"context"
	"fmt"
	"strings"

	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"

	appconfig "github.com/ron/ecsx/pkg"
	ecstypes "github.com/ron/ecsx/pkg/aws/ecs/types"
	ecsadapter "github.com/ron/ecsx/pkg/ui/internal/adapters/envvars"
	"github.com/ron/ecsx/pkg/ui/internal/messages"
	"github.com/ron/ecsx/pkg/ui/internal/styles"
)

type envVarsStyles struct {
	headerStyle    lipgloss.Style
	fieldNameStyle lipgloss.Style
}

type EnvVarsView struct {
	config *appconfig.Config

	viewport viewport.Model
	help     help.Model
	keyMap   envVarsKeyMap
	styles   envVarsStyles

	// data
	containers []ecsadapter.ContainerEnvVars
	format     ecsadapter.Format

	// context
	cluster   string
	service   string
	container string

	// state
	loading bool
	err     error

	// window
	window struct {
		width  int
		height int
	}
}

type envVarsKeyMap struct {
	Format key.Binding
	Copy   key.Binding
	Esc    key.Binding
}

func (k envVarsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Format, k.Copy, k.Esc}
}

func (k envVarsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Format, k.Copy, k.Esc}}
}

func NewEnvVarsView(config *appconfig.Config) *EnvVarsView {
	vp := viewport.New(viewport.WithHeight(20))
	vp.SoftWrap = false
	vp.SetHorizontalStep(5)

	return &EnvVarsView{
		config:   config,
		viewport: vp,
		help:     help.New(),
		styles: envVarsStyles{
			headerStyle:    lipgloss.NewStyle().Bold(true).Foreground(styles.ViewFocusBorderColour).PaddingBottom(1),
			fieldNameStyle: lipgloss.NewStyle().Foreground(styles.SubtleColour),
		},
		keyMap: envVarsKeyMap{
			Format: key.NewBinding(
				key.WithKeys("f"),
				key.WithHelp("f", "format"),
			),
			Copy: key.NewBinding(
				key.WithKeys("y"),
				key.WithHelp("y", "copy"),
			),
			Esc: key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "back"),
			),
		},
	}
}

func (m *EnvVarsView) Init() tea.Cmd {
	return nil
}

func (m *EnvVarsView) Open(cluster, service, container string) tea.Cmd {
	m.cluster = cluster
	m.service = service
	m.container = container
	m.loading = true
	m.err = nil
	m.containers = nil
	m.format = ecsadapter.FormatDetail
	m.viewport.SetContent("Loading environment variables...")

	config := m.config
	return func() tea.Msg {
		ecsClient := config.ECSClient
		if ecsClient == nil {
			return messages.EnvVarsReady{
				Cluster: cluster, Service: service, Container: container,
				Err: fmt.Errorf("ECS client not available"),
			}
		}
		envVars, err := ecsadapter.ResolveEnvVars(ecsClient, context.Background(), cluster, service, container)
		if err != nil {
			return messages.EnvVarsReady{
				Cluster: cluster, Service: service, Container: container,
				Err: err,
			}
		}
		// Store results on the view via a custom message
		return envVarsLoadedMsg{
			cluster:    cluster,
			service:    service,
			container:  container,
			containers: envVars,
		}
	}
}

type envVarsLoadedMsg struct {
	cluster    string
	service    string
	container  string
	containers []ecsadapter.ContainerEnvVars
}

func (m *EnvVarsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case envVarsLoadedMsg:
		m.loading = false
		m.containers = msg.containers
		m.updateContent()
		return nil
	case messages.EnvVarsReady:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.viewport.SetContent(fmt.Sprintf("Error: %s", msg.Err))
		}
		return nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Esc):
			return m.close()
		case key.Matches(msg, m.keyMap.Format):
			m.format = ecsadapter.NextFormat(m.format)
			m.updateContent()
			return nil
		case key.Matches(msg, m.keyMap.Copy):
			return m.copyToClipboard()
		}
	case tea.WindowSizeMsg:
		m.window.width = msg.Width
		m.window.height = msg.Height
		m.applySize()
		return nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return cmd
}

func (m *EnvVarsView) close() tea.Cmd {
	return func() tea.Msg {
		return messages.CloseEnvVars{}
	}
}

func (m *EnvVarsView) copyToClipboard() tea.Cmd {
	if len(m.containers) == 0 {
		return nil
	}
	// When in detail (styled) format, copy as plain table instead of ANSI codes
	copyFormat := m.format
	if copyFormat == ecsadapter.FormatDetail {
		copyFormat = ecsadapter.FormatExport
	}
	var b strings.Builder
	for i, c := range m.containers {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if len(m.containers) > 1 {
			fmt.Fprintf(&b, "# %s\n", c.Container)
		}
		b.WriteString(c.Formatted[copyFormat])
	}
	text := b.String()
	if err := clipboard.WriteAll(text); err != nil {
		return func() tea.Msg {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("clipboard: %w", err),
			}
		}
	}
	return func() tea.Msg {
		return messages.ToggleNotificationDialog{
			Msg:      fmt.Sprintf("Copied %d vars (%s)", m.totalVars(), copyFormat),
			Duration: 2 * time.Second,
		}
	}
}

func (m *EnvVarsView) totalVars() int {
	total := 0
	for _, c := range m.containers {
		total += len(c.EnvVars)
	}
	return total
}

func (m *EnvVarsView) updateContent() {
	if len(m.containers) == 0 {
		return
	}

	if m.format == ecsadapter.FormatDetail {
		// Render styled detail view matching the app's detail pane pattern
		m.viewport.SetContent(m.renderDetailView())
		return
	}

	// Plain text formats
	var b strings.Builder
	for i, c := range m.containers {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if len(m.containers) > 1 {
			fmt.Fprintf(&b, "# %s\n", c.Container)
		}
		b.WriteString(c.Formatted[m.format])
	}
	m.viewport.SetContent(b.String())
}

func (m *EnvVarsView) renderDetailView() string {
	header := m.styles.headerStyle.Render
	field := m.styles.fieldNameStyle.Render

	var b strings.Builder
	for i, c := range m.containers {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s\n", header(strings.ToUpper(c.Container)))
		if len(c.EnvVars) == 0 {
			b.WriteString("  (no environment variables)\n")
			continue
		}
		maxLen := maxNameLen(c.EnvVars)
		for _, ev := range c.EnvVars {
			fmt.Fprintf(&b, "%s:  %s\n", field(fmt.Sprintf("%-*s", maxLen, ev.Name)), ev.Value)
		}
	}
	return b.String()
}

func maxNameLen(vars []ecstypes.EnvVar) int {
	max := 0
	for _, ev := range vars {
		if len(ev.Name) > max {
			max = len(ev.Name)
		}
	}
	return max
}

func (m *EnvVarsView) ApplySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.applySize()
}

func (m *EnvVarsView) applySize() {
	// Reserve 2 lines for status bar + help
	vpHeight := m.window.height - 2
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.SetHeight(vpHeight)
	m.viewport.SetWidth(m.window.width)
	m.help.SetWidth(m.window.width)
}

func (m *EnvVarsView) ShortHelp() []key.Binding {
	return m.keyMap.ShortHelp()
}

func (m *EnvVarsView) View() string {
	statusStyle := lipgloss.NewStyle().
		Foreground(styles.SubtleColour).
		PaddingLeft(1)

	var status string
	if m.loading {
		status = statusStyle.Render("loading...")
	} else if m.err != nil {
		status = statusStyle.Render(fmt.Sprintf("error: %s", m.err))
	} else {
		formatBadge := lipgloss.NewStyle().
			Background(styles.ViewFocusBorderColour).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Render(strings.ToUpper(m.format.String()))

		parts := []string{
			fmt.Sprintf("%s/%s", m.service, m.container),
			fmt.Sprintf("%d vars", m.totalVars()),
			formatBadge,
		}
		status = statusStyle.Render(strings.Join(parts, " │ "))
	}

	helpView := lipgloss.NewStyle().PaddingLeft(1).Render(m.help.View(m.keyMap))

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		status,
		helpView,
	)
}
