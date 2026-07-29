package dialogs

import (
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
	commonstyles "github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
)

type logsField int

const (
	logsFieldCommand logsField = iota
	logsFieldPeriod
	logsFieldFilter
	logsFieldCount // sentinel for wrapping
)

// LogsCommandEditor allows the user to edit the logs viewer command, period, and filter before launching.
type LogsCommandEditor struct {
	styles logsCommandStyles
	keyMap logsCommandKeyMap

	defaultDialogHeight int
	defaultDialogWidth  int
	window              struct {
		width  int
		height int
	}
	dialog struct {
		width  int
		height int
	}

	commandInput textinput.Model
	periodInput  textinput.Model
	filterInput  textinput.Model

	focused logsField

	help help.Model

	// context for the logs request
	cluster   string
	service   string
	container string
}

type logsCommandKeyMap struct {
	enter key.Binding
	close key.Binding
	tab   key.Binding
	stab  key.Binding
}

func (h logsCommandKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{h.tab, h.enter, h.close}
}

func (h logsCommandKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{h.tab, h.stab, h.enter, h.close}}
}

type logsCommandStyles struct {
	dialogStyle lipgloss.Style
	title       lipgloss.Style
	label       lipgloss.Style
	labelFocus  lipgloss.Style
	inputBox    lipgloss.Style
	inputFocus  lipgloss.Style
	helpLine    lipgloss.Style
}

func newLogsCommandStyles() logsCommandStyles {
	return logsCommandStyles{
		dialogStyle: commonstyles.DialogStyle,
		title:       lipgloss.NewStyle().Padding(1, 0, 1, 0),
		label:       lipgloss.NewStyle().Foreground(styles.SubtleColour),
		labelFocus:  lipgloss.NewStyle().Foreground(commonstyles.DialogFocusColour).Bold(true),
		inputBox:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.SubtleColour),
		inputFocus:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(commonstyles.DialogFocusColour),
		helpLine:    lipgloss.NewStyle().Padding(1, 4, 0, 4),
	}
}

func NewLogsCommandEditor() *LogsCommandEditor {
	d := &LogsCommandEditor{}

	d.keyMap = logsCommandKeyMap{
		enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "run"),
		),
		close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next field"),
		),
		stab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev field"),
		),
	}

	d.defaultDialogHeight = 16
	d.defaultDialogWidth = 60

	d.dialog.width = d.defaultDialogWidth
	d.dialog.height = d.defaultDialogHeight

	d.window.width = 150
	d.window.height = 100

	d.styles = newLogsCommandStyles()

	// Command input
	cmd := textinput.New()
	cmd.Prompt = ""
	cmd.CharLimit = 256
	cmd.SetWidth(50)
	cmd.Placeholder = "hl -L -F"
	d.commandInput = cmd

	// Period input
	period := textinput.New()
	period.Prompt = ""
	period.CharLimit = 20
	period.SetWidth(50)
	period.Placeholder = "5m"
	d.periodInput = period

	// Filter input
	filter := textinput.New()
	filter.Prompt = ""
	filter.CharLimit = 256
	filter.SetWidth(50)
	filter.Placeholder = "{ $.level = \"ERROR\" }"
	d.filterInput = filter

	d.help = help.New()

	return d
}

// Open prepares the dialog with context and pre-fills the fields.
func (m *LogsCommandEditor) Open(cluster, service, container, defaultCmd string) tea.Cmd {
	m.cluster = cluster
	m.service = service
	m.container = container

	m.commandInput.SetValue(defaultCmd)
	m.commandInput.CursorEnd()

	m.periodInput.SetValue("5m")
	m.periodInput.CursorEnd()

	m.filterInput.SetValue("")

	m.focused = logsFieldCommand
	return m.focusCurrent()
}

func (m *LogsCommandEditor) Init() tea.Cmd {
	return nil
}

func (m *LogsCommandEditor) Update(msg tea.Msg) tea.Cmd {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, m.keyMap.close):
			return m.cancel()
		case key.Matches(msg, m.keyMap.enter):
			return m.accept()
		case key.Matches(msg, m.keyMap.tab):
			return m.nextField()
		case key.Matches(msg, m.keyMap.stab):
			return m.prevField()
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.window.width = msg.Width
		m.window.height = msg.Height
		m.updateSize()
		return nil
	default:
		return m.updateFocusedInput(msg)
	}
}

func (m *LogsCommandEditor) focusCurrent() tea.Cmd {
	m.commandInput.Blur()
	m.periodInput.Blur()
	m.filterInput.Blur()

	switch m.focused {
	case logsFieldCommand:
		return m.commandInput.Focus()
	case logsFieldPeriod:
		return m.periodInput.Focus()
	case logsFieldFilter:
		return m.filterInput.Focus()
	}
	return nil
}

func (m *LogsCommandEditor) nextField() tea.Cmd {
	m.focused = (m.focused + 1) % logsFieldCount
	return m.focusCurrent()
}

func (m *LogsCommandEditor) prevField() tea.Cmd {
	m.focused = (m.focused - 1 + logsFieldCount) % logsFieldCount
	return m.focusCurrent()
}

func (m *LogsCommandEditor) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focused {
	case logsFieldCommand:
		m.commandInput, cmd = m.commandInput.Update(msg)
	case logsFieldPeriod:
		m.periodInput, cmd = m.periodInput.Update(msg)
	case logsFieldFilter:
		m.filterInput, cmd = m.filterInput.Update(msg)
	}
	return cmd
}

func (m *LogsCommandEditor) cancel() tea.Cmd {
	return func() tea.Msg {
		return messages.CloseLogsCommandEditor{}
	}
}

func (m *LogsCommandEditor) accept() tea.Cmd {
	command := m.commandInput.Value()
	periodStr := m.periodInput.Value()
	filterPattern := m.filterInput.Value()
	cluster := m.cluster
	service := m.service
	container := m.container

	period, err := time.ParseDuration(periodStr)
	if err != nil || period <= 0 {
		period = 5 * time.Minute
	}

	return tea.Batch(
		func() tea.Msg { return messages.CloseLogsCommandEditor{} },
		func() tea.Msg {
			return messages.RunLogsWithCommand{
				Cluster:       cluster,
				Service:       service,
				Container:     container,
				Command:       command,
				Period:        period,
				FilterPattern: filterPattern,
			}
		},
	)
}

func (m *LogsCommandEditor) updateSize() {
	s := newLogsCommandStyles()
	s.dialogStyle = s.dialogStyle.Width(m.dialog.width).Height(m.dialog.height)
	inputW := m.dialog.width - 8
	s.inputBox = s.inputBox.Width(inputW)
	s.inputFocus = s.inputFocus.Width(inputW)
	m.styles = s

	fieldW := m.dialog.width - 12
	m.commandInput.SetWidth(fieldW)
	m.periodInput.SetWidth(fieldW)
	m.filterInput.SetWidth(fieldW)
}

func (m *LogsCommandEditor) renderField(label string, input textinput.Model, field logsField) string {
	var labelStyle, boxStyle lipgloss.Style
	if m.focused == field {
		labelStyle = m.styles.labelFocus
		boxStyle = m.styles.inputFocus
	} else {
		labelStyle = m.styles.label
		boxStyle = m.styles.inputBox
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(label),
		boxStyle.Render(input.View()),
	)
}

func (m *LogsCommandEditor) View() string {
	title := "Logs Viewer Options"

	fields := lipgloss.JoinVertical(lipgloss.Left,
		m.renderField("Command", m.commandInput, logsFieldCommand),
		m.renderField("Period (e.g. 5m, 15m, 1h)", m.periodInput, logsFieldPeriod),
		m.renderField("Filter (CloudWatch pattern)", m.filterInput, logsFieldFilter),
	)

	return m.styles.dialogStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			m.styles.title.Render(title),
			fields,
			m.styles.helpLine.Render(m.help.View(m.keyMap)),
		),
	)
}
