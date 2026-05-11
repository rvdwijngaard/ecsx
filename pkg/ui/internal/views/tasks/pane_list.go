package tasks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	ecsadapter "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs"
	apitypes "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs/types"
	"github.com/ron/ecsx/pkg/ui/internal/components/search"
	"github.com/ron/ecsx/pkg/ui/internal/components/table"
	commonstyles "github.com/ron/ecsx/pkg/ui/internal/styles"
	u "github.com/ron/ecsx/pkg/util"
)

type listPane struct {
	ctx    context.Context
	client *ecs.Client

	// spinner
	spinner struct {
		active bool
		model  spinner.Model
		text   string
	}

	// standard timeout
	stdTO time.Duration

	// error
	err error

	// state
	cluster string
	service string
	tasks   []apitypes.TaskItem

	// pane's view window
	window struct {
		width  int
		height int
	}

	// fuzzy finding
	search *search.SearchBox

	// filtering parameters
	filtering struct {
		matched      []int
		matchedRunes [][]int
		enabled      bool
	}

	// key map
	KeyMap *ListPaneKeyMap

	// the underlying table
	content *table.Model

	// index of last previewed task
	lastPreview int
}

func newListPane(ctx context.Context, client *ecs.Client) *listPane {
	p := &listPane{
		ctx:    ctx,
		client: client,
		stdTO:  30 * time.Second,
		KeyMap: DefaultListPaneKeyMap(),
	}

	{ // contents table
		t := table.New(
			table.WithColumns([]table.Column{
				{Title: "Task ID", Width: 36},
				{Title: "Status", Width: 10},
				{Title: "Health", Width: 10},
				{Title: "Launch", Width: 10},
				{Title: "Started", Width: 20},
			}),
			table.WithFocused(true),
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
		p.content = t
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
					p.filtering.matched = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					return p.MaybePreviewItem(true)
				},
				Results: func(_ string, results []search.FilteredItem) tea.Cmd {
					p.filtering.enabled = true
					p.filtering.matched = make([]int, len(results))
					p.filtering.matchedRunes = make([][]int, len(results))
					rows := p.content.Rows()
					filtered := make([]table.Row, len(results))
					for i, match := range results {
						filtered[i] = rows[match.Index]
						p.filtering.matched[i] = match.Index
						p.filtering.matchedRunes[i] = match.Matches
					}
					p.content.SetVirtualRows(filtered)
					return nil
				},
				Reset: func(searchHeight int) tea.Cmd {
					p.filtering.enabled = false
					p.filtering.matched = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					p.updateSize()
					return p.MaybePreviewItem(true)
				},
				SearchBoxOpens: func(searchHeight int) tea.Cmd {
					p.updateSize()
					return nil
				},
			},
		)
	}

	return p
}

func (m *listPane) Load(cluster, service string) tea.Cmd {
	m.err = nil
	m.cluster = cluster
	m.service = service
	m.tasks = nil
	m.lastPreview = -1
	m.content.SetCursor(0)
	m.search.Reset()
	m.content.ResetVirtualRows()
	return m.loadTasks()
}

func (m *listPane) loadTasks() tea.Cmd {
	spinnerCmd := m.activateSpinner()
	client := m.client
	cluster := m.cluster
	service := m.service
	ctx, cancel := context.WithTimeout(m.ctx, m.stdTO)
	load := func() tea.Msg {
		defer cancel()
		tasks, err := ecsadapter.ListTasks(client, ctx, cluster, service)
		return tasksReadyMsg{tasks: tasks, err: err}
	}
	return tea.Batch(load, spinnerCmd)
}

func (m *listPane) activateSpinner() tea.Cmd {
	m.spinner.active = true
	m.updateSize()
	return m.spinner.model.Tick
}

func (m *listPane) deactivateSpinner() {
	m.spinner.active = false
	m.updateSize()
}

func (m *listPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tasksReadyMsg:
		m.deactivateSpinner()
		if msg.err != nil {
			m.err = msg.err
			return nil
		}
		m.tasks = msg.tasks
		m.setRows()
		return m.MaybePreviewItem(true)

	case spinner.TickMsg:
		if !m.spinner.active {
			return nil
		}
		var cmd tea.Cmd
		m.spinner.model, cmd = m.spinner.model.Update(msg)
		return cmd
	}

	// route to search when applicable
	if search.IsSearchBoxMessage(msg) || m.search.IsFocused() {
		cmd := m.search.Update(msg)
		return tea.Batch(cmd, m.MaybePreviewItem(false))
	}

	// handle navigation
	if _, isKey := msg.(tea.KeyPressMsg); isKey {
		return m.handleKey(msg.(tea.KeyPressMsg))
	}

	cmd := m.content.Update(msg)
	return tea.Batch(cmd, m.MaybePreviewItem(false))
}

func (m *listPane) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.KeyMap.Quit):
		return tea.Quit
	case key.Matches(msg, m.KeyMap.Search):
		return m.search.OpenSearchBox()
	case key.Matches(msg, m.KeyMap.Back):
		return m.goBack()
	case key.Matches(msg, m.KeyMap.Reload):
		return m.Load(m.cluster, m.service)
	case key.Matches(msg, m.KeyMap.Zoom):
		return m.zoom()
	}
	cmd := m.content.Update(msg)
	return tea.Batch(cmd, m.MaybePreviewItem(false))
}

func (m *listPane) goBack() tea.Cmd {
	return func() tea.Msg {
		return BackToServicesMsg{Cluster: m.cluster}
	}
}

func (m *listPane) zoom() tea.Cmd {
	return func() tea.Msg {
		return zoomToggleListPane{}
	}
}

// MaybePreviewItem sends a details message if the cursor moved.
func (m *listPane) MaybePreviewItem(force bool) tea.Cmd {
	if len(m.tasks) == 0 {
		return func() tea.Msg {
			return taskDetailsMsg{task: nil}
		}
	}
	idx := m.content.Cursor()
	if m.filtering.enabled {
		if len(m.filtering.matched) == 0 {
			return func() tea.Msg {
				return taskDetailsMsg{task: nil}
			}
		}
		if idx >= len(m.filtering.matched) {
			idx = len(m.filtering.matched) - 1
		}
		idx = m.filtering.matched[idx]
	}
	if idx == m.lastPreview && !force {
		return nil
	}
	m.lastPreview = idx
	if idx < 0 || idx >= len(m.tasks) {
		return nil
	}
	t := m.tasks[idx]
	return func() tea.Msg {
		return taskDetailsMsg{task: &t}
	}
}

func (m *listPane) setRows() {
	rows := make([]table.Row, len(m.tasks))
	for i, t := range m.tasks {
		started := ""
		if t.StartedAt != nil {
			started = t.StartedAt.Local().Format("2006-01-02 15:04:05")
		}
		rows[i] = table.Row{
			simpleField{t.ID},
			simpleField{t.Status},
			simpleField{t.HealthStatus},
			simpleField{t.LaunchType},
			simpleField{started},
		}
	}
	m.content.SetRows(rows)
}

func (m *listPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.updateSize()
}

// updateSize recalculates content dimensions based on current state.
func (m *listPane) updateSize() {
	h, w := m.window.height, m.window.width
	searchBoxH := u.Ternary(m.search.GetHeight(), 0, m.search.IsEnabled())
	spinnerH := u.Ternary(1, 0, m.spinner.active)
	m.content.SetHeight(h - searchBoxH - spinnerH)
	m.content.SetWidth(w)
	m.search.SetWidth(w)
}

func (m *listPane) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	content := u.Ternary(m.content.View(), m.noContentMessage(), len(m.content.Rows()) > 0)
	rendering := []string{content}
	if m.spinner.active {
		rendering = append(rendering, fmt.Sprintf("%s %s", m.spinner.model.View(), m.spinner.text))
	}
	rendering = append(rendering, m.search.View())
	return lipgloss.JoinVertical(lipgloss.Left, rendering...)
}

func (m *listPane) noContentMessage() string {
	if m.spinner.active {
		return ""
	}
	s := strings.Builder{}
	fmt.Fprintf(&s, "==================================================\n")
	fmt.Fprintf(&s, "       NO TASKS IN %s / %s\n", m.cluster, m.service)
	fmt.Fprintf(&s, "==================================================\n")
	return s.String()
}

// Messages
type tasksReadyMsg struct {
	tasks []apitypes.TaskItem
	err   error
}

type taskDetailsMsg struct {
	task *apitypes.TaskItem
}

type zoomToggleListPane struct{}

// simpleField implements table.Field
type simpleField struct {
	value string
}

func (f simpleField) Value() string { return f.value }
