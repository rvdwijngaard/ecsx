package dialogs

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	commonstyles "github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
)

type containerPickerKeyMap struct {
	close key.Binding
	enter key.Binding
}

func (h containerPickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{h.close, h.enter}
}

// ContainerPickerPurpose determines what action to take after container selection.
type ContainerPickerPurpose int

const (
	PickerPurposeLogs ContainerPickerPurpose = iota
	PickerPurposeEnvVars
)

// ContainerPicker enables the user to select a container for log viewing.
type ContainerPicker struct {
	keyMap containerPickerKeyMap

	defaultDialogHeight int
	defaultDialogWidth  int

	window struct {
		width  int
		height int
	}

	// context for the request
	cluster string
	service string
	purpose ContainerPickerPurpose

	// available containers
	containers []string

	styles containerPickerStyles

	content list.Model
}

type containerPickerStyles struct {
	dialog   lipgloss.Style
	title    lipgloss.Style
	content  lipgloss.Style
	help     lipgloss.Style
	helpLine lipgloss.Style
}

func newContainerPickerStyles() containerPickerStyles {
	return containerPickerStyles{
		dialog:   commonstyles.DialogStyle,
		title:    lipgloss.NewStyle().Padding(1, 0, 1, 0),
		content:  lipgloss.NewStyle().Padding(1, 0, 1, 0),
		help:     lipgloss.NewStyle().Padding(1, 2, 0, 2),
		helpLine: lipgloss.NewStyle().PaddingBottom(1),
	}
}

// containerItem implements list.Item for the picker.
type containerItem struct {
	name string
}

func (i containerItem) FilterValue() string { return i.name }

// containerItemDelegate renders container items in the list.
type containerItemDelegate struct{}

func (d containerItemDelegate) Height() int                             { return 1 }
func (d containerItemDelegate) Spacing() int                            { return 0 }
func (d containerItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d containerItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(containerItem)
	if !ok {
		return
	}

	str := i.name
	if index == m.Index() {
		fmt.Fprint(w, lipgloss.NewStyle().PaddingLeft(2).Foreground(commonstyles.DialogFocusColour).Render("> "+str))
	} else {
		fmt.Fprint(w, lipgloss.NewStyle().PaddingLeft(4).Render(str))
	}
}

// NewContainerPicker creates a new container picker dialog.
func NewContainerPicker() *ContainerPicker {
	p := &ContainerPicker{
		keyMap: containerPickerKeyMap{
			close: key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "cancel"),
			),
			enter: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
		},
		defaultDialogHeight: 20,
		defaultDialogWidth:  40,
	}

	p.window.width = 150
	p.window.height = 100

	l := list.New([]list.Item{}, containerItemDelegate{}, p.defaultDialogWidth, p.defaultDialogHeight)
	l.Title = "Select Container"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.KeyMap.Quit.SetEnabled(false)

	p.content = l
	p.styles = newContainerPickerStyles()

	return p
}

// SetContainers configures the dialog with available containers and context.
func (m *ContainerPicker) SetContainers(cluster, service string, containers []string, purpose ContainerPickerPurpose) {
	m.cluster = cluster
	m.service = service
	m.containers = containers
	m.purpose = purpose

	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = containerItem{name: c}
	}
	m.content.SetItems(items)
	m.content.Select(0)
	m.updateSize()
}

func (m *ContainerPicker) Init() tea.Cmd {
	return nil
}

func (m *ContainerPicker) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.close):
			return m.close()
		case key.Matches(msg, m.keyMap.enter):
			return m.selectContainer()
		default:
			var cmd tea.Cmd
			m.content, cmd = m.content.Update(msg)
			return cmd
		}
	case tea.WindowSizeMsg:
		m.window.width = msg.Width
		m.window.height = msg.Height
		m.updateSize()
		return nil
	}
	return nil
}

func (m *ContainerPicker) selectContainer() tea.Cmd {
	itm := m.content.SelectedItem()
	if itm == nil {
		return m.close()
	}
	selected := itm.(containerItem).name
	cluster := m.cluster
	service := m.service
	purpose := m.purpose
	return tea.Batch(
		m.close(),
		func() tea.Msg {
			switch purpose {
			case PickerPurposeEnvVars:
				return messages.OpenEnvVars{
					Cluster:   cluster,
					Service:   service,
					Container: selected,
				}
			default:
				return messages.OpenLogs{
					Cluster:   cluster,
					Service:   service,
					Container: selected,
				}
			}
		},
	)
}

func (m *ContainerPicker) close() tea.Cmd {
	return func() tea.Msg {
		return messages.CloseContainerPicker{}
	}
}

func (m *ContainerPicker) updateSize() {
	items := m.content.Items()

	padding := 4
	m.content.SetHeight(min(len(items)+padding, m.window.height-10))

	width := m.defaultDialogWidth
	for _, itm := range items {
		w := len(itm.(containerItem).name) + 6
		width = max(width, w)
	}
	m.content.SetWidth(width)

	m.styles.dialog = m.styles.dialog.
		Height(m.content.Height() + 4).
		Width(width + 4)
}

func (m *ContainerPicker) View() string {
	title := m.styles.title.Render(m.content.Title)
	content := m.styles.content.Render(m.content.View())
	help := m.styles.help.Render(
		m.styles.helpLine.Render(
			strings.Join([]string{
				m.keyMap.enter.Help().Key + " " + m.keyMap.enter.Help().Desc,
				m.keyMap.close.Help().Key + " " + m.keyMap.close.Help().Desc,
			}, " • "),
		),
	)
	return m.styles.dialog.Render(
		lipgloss.JoinVertical(lipgloss.Center, title, content, help),
	)
}
