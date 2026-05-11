package clusters

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
	"github.com/ron/ecsx/pkg/ui/internal/messages"
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

	// the clusters retrieved from ECS
	clusters []apitypes.ClusterItem

	// index of last previewed cluster
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
				{Title: "Cluster", Width: 40},
				{Title: "Services", Width: 10},
				{Title: "Running", Width: 9},
				{Title: "Pending", Width: 9},
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
		p.spinner.text = "loading clusters..."
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

func (m *listPane) Init() tea.Cmd {
	m.err = nil
	m.clusters = nil
	m.lastPreview = -1
	m.content.SetCursor(0)
	m.search.Reset()
	m.content.ResetVirtualRows()
	return m.loadClusters()
}

func (m *listPane) loadClusters() tea.Cmd {
	spinnerCmd := m.activateSpinner()
	client := m.client
	ctx, cancel := context.WithTimeout(m.ctx, m.stdTO)
	load := func() tea.Msg {
		defer cancel()
		clusters, err := ecsadapter.ListClusters(client, ctx)
		return clustersReadyMsg{clusters: clusters, err: err}
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
	case clustersReadyMsg:
		m.deactivateSpinner()
		if msg.err != nil {
			m.err = msg.err
			return nil
		}
		m.clusters = msg.clusters
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

	if search.IsSearchBoxMessage(msg) || m.search.IsFocused() {
		cmd := m.search.Update(msg)
		return tea.Batch(cmd, m.MaybePreviewItem(false))
	}

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
	case key.Matches(msg, m.KeyMap.Esc):
		return m.search.Reset()
	case key.Matches(msg, m.KeyMap.Select):
		return m.selectCluster()
	case key.Matches(msg, m.KeyMap.Reload):
		return m.Init()
	case key.Matches(msg, m.KeyMap.Zoom):
		return m.zoom()
	}
	cmd := m.content.Update(msg)
	return tea.Batch(cmd, m.MaybePreviewItem(false))
}

func (m *listPane) selectCluster() tea.Cmd {
	idx := m.content.Cursor()
	if m.filtering.enabled {
		if idx < 0 || idx >= len(m.filtering.matched) {
			return nil
		}
		idx = m.filtering.matched[idx]
	}
	if idx < 0 || idx >= len(m.clusters) {
		return nil
	}
	cluster := m.clusters[idx]
	return func() tea.Msg {
		return messages.SelectCluster{ClusterName: cluster.Name}
	}
}

func (m *listPane) zoom() tea.Cmd {
	return func() tea.Msg {
		return zoomToggleListPane{}
	}
}

func (m *listPane) MaybePreviewItem(force bool) tea.Cmd {
	if len(m.clusters) == 0 {
		return func() tea.Msg {
			return clusterDetailsMsg{cluster: nil}
		}
	}
	idx := m.content.Cursor()
	if m.filtering.enabled {
		if len(m.filtering.matched) == 0 {
			return func() tea.Msg {
				return clusterDetailsMsg{cluster: nil}
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
	if idx < 0 || idx >= len(m.clusters) {
		return nil
	}
	c := m.clusters[idx]
	return func() tea.Msg {
		return clusterDetailsMsg{cluster: &c}
	}
}

func (m *listPane) setRows() {
	rows := make([]table.Row, len(m.clusters))
	for i, c := range m.clusters {
		rows[i] = table.Row{
			simpleField{c.Name},
			simpleField{fmt.Sprintf("%d", c.ActiveServices)},
			simpleField{fmt.Sprintf("%d", c.RunningTasks)},
			simpleField{fmt.Sprintf("%d", c.PendingTasks)},
		}
	}
	m.content.SetRows(rows)
}

func (m *listPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.updateSize()
}

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
	fmt.Fprintf(&s, "              NO CLUSTERS AVAILABLE               \n")
	fmt.Fprintf(&s, "==================================================\n")
	return s.String()
}

// Messages
type clustersReadyMsg struct {
	clusters []apitypes.ClusterItem
	err      error
}

type clusterDetailsMsg struct {
	cluster *apitypes.ClusterItem
}

type zoomToggleListPane struct{}

// simpleField implements table.Field
type simpleField struct {
	value string
}

func (f simpleField) Value() string { return f.value }
