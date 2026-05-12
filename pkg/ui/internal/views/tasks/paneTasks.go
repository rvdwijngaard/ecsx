package taskselection

import (
	"context"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"

	appconfig "github.com/ron/ecsx/pkg"
	ecsadapter "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs"
	apitypes "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs/types"
	"github.com/ron/ecsx/pkg/ui/internal/components/search"
	"github.com/ron/ecsx/pkg/ui/internal/components/table"
	"github.com/ron/ecsx/pkg/ui/internal/messages"
	commonstyles "github.com/ron/ecsx/pkg/ui/internal/styles"
	"github.com/ron/ecsx/pkg/ui/internal/views/util/keymaps"
	u "github.com/ron/ecsx/pkg/util"
)

type TaskTableStyles struct {
	SelectedBackground    color.Color
	SearchMatchBackground color.Color
}

type taskSelectionPane struct {
	// top-level context
	ctx context.Context

	// shared config (ECS client accessed via config.ECSClient)
	config *appconfig.Config

	// styles
	styles struct {
		Table TaskTableStyles
	}

	// spinner
	spinner struct {
		active bool
		model  spinner.Model
		text   string
	}

	// cancel last call context (debounce)
	cancelDetails func()
	debounceDur   time.Duration

	// cancel loading tasks
	cancelTasks func()

	// standard timeout
	stdTO time.Duration

	// errorText
	err error

	// pane's view window
	window struct {
		width  int
		height int
	}

	// fuzzy finding
	search *search.SearchBox

	// key map
	KeyMap *TaskPaneKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	// the underlying table component
	content *table.Model

	// the tasks retrieved from ECS
	tasks []apitypes.TaskItem

	// the currently selected cluster and service
	clusterName string
	serviceName string

	// filtering parameters
	filtering struct {
		matchedTasks []int   // indices referring to tasks
		matchedRunes [][]int // matches by index to filtering.matchedTasks
		enabled      bool
	}

	// index to most recently previewed task
	lastTaskDetails int
}

type taskPaneOption func(p *taskSelectionPane)

func withTaskPaneKeys(keys keymaps.AdditionalKeys) taskPaneOption {
	return func(t *taskSelectionPane) {
		t.AddKeyMap = keys
	}
}

func newTaskSelectionPane(ctx context.Context, config *appconfig.Config, opts ...taskPaneOption) *taskSelectionPane {
	p := &taskSelectionPane{
		ctx:           ctx,
		config:        config,
		cancelDetails: func() {}, // noop on init
		cancelTasks:   func() {}, // noop on init
		debounceDur:   50 * time.Millisecond,
		stdTO:         30 * time.Second,
		KeyMap:        DefaultTaskPaneKeyMap(),
	}

	{ // contents table
		t := table.New(
			table.WithColumns([]table.Column{
				{Title: "task-id", Width: 36},
				{Title: "info", Width: 25},
			}),
			table.WithFocused(true),
			table.WithFieldDelegate(p.TaskRowFieldDelegate),
		)
		s := table.DefaultStyles()
		s.Header = s.Header.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(commonstyles.TableDefaultFg).
			BorderBottom(true).
			Bold(false)
		s.Selected = s.Selected.
			Foreground(commonstyles.TableSelectedFg).
			Background(commonstyles.TableSelectedBg).
			Bold(false)
		t.SetStyles(s)

		st := TaskTableStyles{
			SelectedBackground:    commonstyles.TableSelectedBg,
			SearchMatchBackground: commonstyles.SearchHighlight,
		}

		p.content = t
		p.styles.Table = st
	}

	{ // spinner
		sp := spinner.New()
		sp.Spinner = spinner.Dot
		sp.Style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			PaddingLeft(1)
		p.spinner.model = sp
		p.spinner.text = "loading tasks..."
	}

	{ // search box
		p.search = search.NewSearchBox(
			search.SearchCallbacks{
				ToSearch: func(string) []string {
					return table.Rows(p.content.Rows()).ToStrings()
				},
				EmptyInput: func() tea.Cmd {
					p.filtering.enabled = false
					p.filtering.matchedTasks = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					return p.MaybePreviewTask(true)
				},
				Results: func(_ string, results []search.FilteredItem) tea.Cmd {
					p.filtering.enabled = true
					p.filtering.matchedTasks = make([]int, len(results))
					p.filtering.matchedRunes = make([][]int, len(results))
					rows := p.content.Rows()
					filtered := make([]table.Row, len(results))
					for i, match := range results {
						filtered[i] = rows[match.Index]
						p.filtering.matchedTasks[i] = match.Index
						p.filtering.matchedRunes[i] = match.Matches
					}
					p.content.SetVirtualRows(filtered)
					return nil
				},
				Reset: func(searchHeight int) tea.Cmd {
					p.filtering.enabled = false
					p.filtering.matchedTasks = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					p.updateSize()
					return p.MaybePreviewTask(true)
				},
				SearchBoxOpens: func(searchHeight int) tea.Cmd {
					p.updateSize()
					return nil
				},
			},
		)
	}

	for _, o := range opts {
		o(p)
	}

	if !keymaps.UniqueKeyMaps(p.KeyMap.ShortHelp(), p.AddKeyMap.Bindings()) {
		panic("overlapping keymaps!")
	}

	return p
}

func (m *taskSelectionPane) cleanSlate() {
	m.err = nil
}

func (m *taskSelectionPane) Init() tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.tasks = []apitypes.TaskItem{}
	m.clusterName = ""
	m.serviceName = ""

	// cancel any lingering calls
	m.cancelDetails()
	m.cancelTasks()
	return nil
}

func (m *taskSelectionPane) loadTasks() tea.Cmd {
	spinnerCmd := m.activateSpinner()

	client := m.config.ECSClient
	cluster := m.clusterName
	service := m.serviceName
	ctx, cc := context.WithTimeout(m.ctx, m.stdTO)
	m.cancelTasks = cc

	return tea.Batch(func() tea.Msg {
		defer cc()
		tasks, err := ecsadapter.ListTasks(client, ctx, cluster, service, m.config.Region)
		return messages.TaskPageReady{
			Cluster: cluster,
			Service: service,
			Tasks:   tasks,
			Err:     err,
		}
	}, spinnerCmd)
}

func (m *taskSelectionPane) activateSpinner() tea.Cmd {
	m.spinner.active = true
	m.updateSize()
	return m.spinner.model.Tick
}

func (m *taskSelectionPane) deactivateSpinner() {
	m.spinner.active = false
	m.updateSize()
}

func (m *taskSelectionPane) processTaskPage(msg messages.TaskPageReady) tea.Cmd {
	m.deactivateSpinner()
	if msg.Cluster != m.clusterName || msg.Service != m.serviceName { // expired
		return nil
	}
	if msg.Err != nil {
		m.err = msg.Err
		return nil
	}
	m.tasks = msg.Tasks

	rows := make([]table.Row, len(m.tasks))
	for i, t := range m.tasks {
		info := fmt.Sprintf("%s %s %s", t.LaunchType, t.Status, taskUptime(t.StartedAt))
		rows[i] = []table.Field{
			enrichedField{value: t.ID},
			enrichedField{value: info},
		}
	}
	m.content.SetRows(rows)

	return m.MaybePreviewTask(true)
}

func (m *taskSelectionPane) Update(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case messages.TaskDetails:
		return nil
	case messages.ServiceDetails:
		return nil
	case messages.ClusterDetails:
		return nil
	case messages.ClusterPageReady:
		return nil
	case messages.ServicePageReady:
		return nil
	case messages.SelectService:
		return m.selectService(msg.ClusterName, msg.ServiceName)
	case messages.TaskPageReady:
		return m.processTaskPage(msg)
	case spinner.TickMsg:
		if !m.spinner.active {
			return nil
		}
		var cmd tea.Cmd
		m.spinner.model, cmd = m.spinner.model.Update(msg)
		return cmd
	}

	if search.IsSearchBoxMessage(msg) || m.search.IsFocused() {
		cmds = append(cmds, m.search.Update(msg))
	} else {
		cmds = append(cmds, m.handleNavigation(msg))
	}

	cmds = append(cmds, m.MaybePreviewTask(false))
	return tea.Batch(cmds...)
}

func (m *taskSelectionPane) selectService(clusterName, serviceName string) tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.tasks = []apitypes.TaskItem{}
	m.clusterName = clusterName
	m.serviceName = serviceName
	m.lastTaskDetails = -1

	m.cancelDetails()
	m.cancelTasks()
	return m.loadTasks()
}

// handleNavigation handles events when search is not active.
func (m *taskSelectionPane) handleNavigation(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Search):
			cmds = append(cmds, m.search.OpenSearchBox())
		case key.Matches(msg, m.KeyMap.Zoom):
			return m.Zoom()
		case key.Matches(msg, m.KeyMap.Esc):
			if m.search.IsEnabled() {
				m.search.Reset()
			} else {
				return m.escape()
			}
		case key.Matches(msg, m.KeyMap.Reload):
			return m.reload()
		case key.Matches(msg, m.KeyMap.Copy):
			return m.copy()
		case key.Matches(msg, m.KeyMap.Logs):
			return m.openLogs()
		default:
			if match, call := m.AddKeyMap.Matches(msg); match {
				return call
			}
		}
	}
	cmds = append(cmds, m.content.Update(msg))
	return tea.Batch(cmds...)
}

type enrichedField struct {
	value string
}

// Value implements the table.Field interface.
func (f enrichedField) Value() string {
	return f.value
}

func (m *taskSelectionPane) TaskRowFieldDelegate(row table.Row, col table.Column, colIdx, rowIdx, colW, padL, padR int, selected bool) string {
	fullWidth := colW + padL + padR

	field := row[colIdx].(enrichedField)

	enforceWidth := lipgloss.NewStyle().Width(fullWidth).MaxWidth(fullWidth).Inline(true).Render
	padding := lipgloss.NewStyle().Padding(0, 1).Render

	if !selected && !m.filtering.enabled {
		return padding(enforceWidth(field.value))
	}

	style := commonstyles.LineStyle{}.AppendStringLG(field.value, lipgloss.NewStyle())

	style = style.SetRightPaddingLast(padR)
	style = style.SetLeftPaddingFirst(padL)

	if selected {
		if len([]rune(field.value)) < fullWidth {
			st, _ := style.GetAt(len([]rune(field.value)) - 1)
			style = style.Override(len([]rune(field.value))-1, st.PaddingRight(fullWidth-len([]rune(field.value))))
		}
		style = style.SetBackgroundAll(m.styles.Table.SelectedBackground)
	}

	if m.filtering.enabled {
		for _, idx := range m.filtering.matchedRunes[rowIdx] {
			runeStyle, _ := style.GetAt(idx)
			c := m.styles.Table.SearchMatchBackground
			if selected {
				c = lipgloss.Blend1D(10, c, m.styles.Table.SelectedBackground)[3]
			}
			style = style.Override(idx, runeStyle.Background(c))
		}
	}

	return enforceWidth(style.Render(field.value))
}

func (m *taskSelectionPane) copy() tea.Cmd {
	r := m.content.VisualRows()
	c := max(0, m.content.Cursor())
	if c >= len(r) {
		return nil
	}

	if err := clipboard.WriteAll(r[c].String()); err != nil {
		return func() tea.Msg {
			return messages.ToggleNotificationDialog{Error: fmt.Errorf("failed to copy: %w", err)}
		}
	}
	return notifyCopySuccess
}

// openLogs emits an OpenLogs message for the current service.
func (m *taskSelectionPane) openLogs() tea.Cmd {
	if m.clusterName == "" || m.serviceName == "" {
		return nil
	}
	cluster := m.clusterName
	service := m.serviceName
	return func() tea.Msg {
		return messages.OpenLogs{
			Cluster:   cluster,
			Service:   service,
			Container: "", // pick first available
		}
	}
}

// MaybePreviewTask sends a TaskDetails message for the currently selected task.
func (m *taskSelectionPane) MaybePreviewTask(force bool) tea.Cmd {
	if len(m.tasks) == 0 || (m.filtering.enabled && len(m.filtering.matchedTasks) == 0) {
		if m.lastTaskDetails == -1 && !force {
			return nil
		}
		m.lastTaskDetails = -1
		return func() tea.Msg {
			return messages.TaskDetails{
				Details: nil,
			}
		}
	}

	idx := m.content.Cursor()
	if m.filtering.enabled && len(m.filtering.matchedTasks) > 0 {
		idx = m.filtering.matchedTasks[idx]
	}
	if idx == m.lastTaskDetails && !force {
		return nil
	}
	m.lastTaskDetails = idx
	task := m.tasks[idx]

	return func() tea.Msg {
		return messages.TaskDetails{
			Details: &task,
		}
	}
}

func (m *taskSelectionPane) Zoom() tea.Cmd {
	return func() tea.Msg {
		return messages.ZoomToggleTaskSelectionPane{}
	}
}

func (m *taskSelectionPane) escape() tea.Cmd {
	m.cancelDetails()
	m.cancelTasks()

	switchView := func() tea.Msg {
		return messages.SwitchView{
			OldView: messages.Task_selection,
			NewView: messages.Service_selection,
		}
	}
	resetPreview := func() tea.Msg {
		return messages.TaskDetails{Details: nil}
	}
	return tea.Batch(switchView, resetPreview)
}

func (m *taskSelectionPane) reload() tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.tasks = []apitypes.TaskItem{}
	m.lastTaskDetails = -1

	m.cancelDetails()
	m.cancelTasks()
	return m.loadTasks()
}

func (m *taskSelectionPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.updateSize()
}

func (m *taskSelectionPane) updateSize() {
	h, w := m.window.height, m.window.width

	searchBoxH := u.Ternary(m.search.GetHeight(), 0, m.search.IsEnabled())
	m.content.SetHeight(h - searchBoxH - u.Ternary(1, 0, m.spinner.active))
	m.content.SetWidth(w)
	m.search.SetWidth(w)
}

func (m *taskSelectionPane) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	content := u.Ternary(m.content.View(), m.noContentMessage(), len(m.content.Rows()) > 0)
	rendering := []string{content, m.search.View()}
	if m.spinner.active {
		rendering = slices.Insert(rendering, 1, fmt.Sprintf("%s %s", m.spinner.model.View(), m.spinner.text))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rendering...)
}

func (m *taskSelectionPane) noContentMessage() string {
	if m.spinner.active {
		return ""
	}
	s := strings.Builder{}
	fmt.Fprintf(&s, "==================================================\n")
	fmt.Fprintf(&s, "          NO TASKS FOR THIS SERVICE               \n")
	fmt.Fprintf(&s, "==================================================\n")
	return s.String()
}

func notifyCopySuccess() tea.Msg {
	return messages.ToggleNotificationDialog{Msg: "Copied!", Duration: 1 * time.Second}
}

func taskUptime(started *time.Time) string {
	if started == nil {
		return ""
	}
	d := time.Since(*started)
	if d < 0 {
		d = 0
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("up %dd%dh%dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("up %dh%dm", hours, minutes)
	}
	return fmt.Sprintf("up %dm", minutes)
}
