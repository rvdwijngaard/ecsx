package clusterselection

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

	appconfig "github.com/rvdwijngaard/ecsx/pkg"
	ecsadapter "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs"
	apitypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs/types"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/components/search"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/components/table"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	commonstyles "github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/util/keymaps"
	u "github.com/rvdwijngaard/ecsx/pkg/util"
)

type ClusterTableStyles struct {
	SelectedBackground    color.Color
	SearchMatchBackground color.Color
}

type clusterSelectionPane struct {
	// top-level context
	ctx context.Context

	// shared config (ECS client accessed via config.ECSClient)
	config *appconfig.Config

	// styles
	styles struct {
		Table ClusterTableStyles
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
	KeyMap *ClusterPaneKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	// the underlying table component
	content *table.Model

	// the clusters retrieved from ECS
	clusters []apitypes.ClusterItem

	// filtering parameters
	filtering struct {
		matchedClusters []int   // indices referring to clusters
		matchedRunes    [][]int // matches by index to filtering.matchedClusters
		enabled         bool
	}

	// index to most recently previewed cluster
	lastClusterDetails int
}

type clusterPaneOption func(p *clusterSelectionPane)

func withClusterPaneKeys(keys keymaps.AdditionalKeys) clusterPaneOption {
	return func(t *clusterSelectionPane) {
		t.AddKeyMap = keys
	}
}

func newClusterSelectionPane(ctx context.Context, config *appconfig.Config, opts ...clusterPaneOption) *clusterSelectionPane {
	p := &clusterSelectionPane{
		ctx:           ctx,
		config:        config,
		cancelDetails: func() {}, // noop on init
		debounceDur:   50 * time.Millisecond,
		stdTO:         30 * time.Second,
		KeyMap:        DefaultClusterPaneKeyMap(),
	}

	{ // contents table
		t := table.New(
			table.WithColumns([]table.Column{{Title: "cluster-name", Width: 64}}),
			table.WithFocused(true),
			table.WithFieldDelegate(p.ClusterRowFieldDelegate),
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

		st := ClusterTableStyles{
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
					p.filtering.matchedClusters = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					return p.MaybePreviewCluster(true)
				},
				Results: func(_ string, results []search.FilteredItem) tea.Cmd {
					p.filtering.enabled = true
					p.filtering.matchedClusters = make([]int, len(results))
					p.filtering.matchedRunes = make([][]int, len(results))
					rows := p.content.Rows()
					filtered := make([]table.Row, len(results))
					for i, match := range results {
						filtered[i] = rows[match.Index]
						p.filtering.matchedClusters[i] = match.Index
						p.filtering.matchedRunes[i] = match.Matches
					}
					p.content.SetVirtualRows(filtered)
					return nil
				},
				Reset: func(searchHeight int) tea.Cmd {
					p.filtering.enabled = false
					p.filtering.matchedClusters = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					p.updateSize()
					return p.MaybePreviewCluster(true)
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

func (m *clusterSelectionPane) cleanSlate() {
	m.err = nil
}

func (m *clusterSelectionPane) Init() tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.clusters = []apitypes.ClusterItem{}

	// cancel any lingering calls
	m.cancelDetails()
	return m.loadClusters()
}

func (m *clusterSelectionPane) loadClusters() tea.Cmd {
	spinnerCmd := m.activateSpinner()
	m.updateSize()

	client := m.config.ECSClient
	ctx, cc := context.WithTimeout(m.ctx, m.stdTO)
	_ = cc // timeout will cancel automatically

	return tea.Batch(func() tea.Msg {
		clusters, err := ecsadapter.ListClusters(client, ctx)
		return messages.ClusterPageReady{
			Clusters: clusters,
			Err:      err,
		}
	}, spinnerCmd)
}

func (m *clusterSelectionPane) activateSpinner() tea.Cmd {
	m.spinner.active = true
	m.updateSize()
	return m.spinner.model.Tick
}

func (m *clusterSelectionPane) deactivateSpinner() {
	m.spinner.active = false
	m.updateSize()
}

func (m *clusterSelectionPane) processClusterPage(msg messages.ClusterPageReady) tea.Cmd {
	m.deactivateSpinner()
	if msg.Err != nil {
		m.err = msg.Err
		return nil
	}
	m.clusters = msg.Clusters

	rows := make([]table.Row, len(m.clusters))
	for i, c := range m.clusters {
		rows[i] = []table.Field{
			enrichedField{value: c.Name},
		}
	}
	m.content.SetRows(rows)

	return m.MaybePreviewCluster(true)
}

func (m *clusterSelectionPane) Update(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case messages.ClusterDetails:
		return nil
	case messages.ServiceDetails:
		return nil
	case messages.TaskDetails:
		return nil
	case messages.ServicePageReady:
		return nil
	case messages.TaskPageReady:
		return nil
	case messages.ClusterPageReady:
		return m.processClusterPage(msg)
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

	cmds = append(cmds, m.MaybePreviewCluster(false))
	return tea.Batch(cmds...)
}

// handleNavigation handles events when search is not active.
func (m *clusterSelectionPane) handleNavigation(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Search):
			cmds = append(cmds, m.search.OpenSearchBox())
		case key.Matches(msg, m.KeyMap.Select):
			return m.selectCluster()
		case key.Matches(msg, m.KeyMap.Zoom):
			return m.Zoom()
		case key.Matches(msg, m.KeyMap.Esc):
			m.search.Reset()
		case key.Matches(msg, m.KeyMap.Reload):
			return m.Init()
		case key.Matches(msg, m.KeyMap.Copy):
			return m.copy()
		case key.Matches(msg, m.KeyMap.OpenConsole):
			return m.openConsole()
		case key.Matches(msg, m.KeyMap.HostShell):
			return m.openHostShell()
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

func (m *clusterSelectionPane) ClusterRowFieldDelegate(row table.Row, col table.Column, colIdx, rowIdx, colW, padL, padR int, selected bool) string {
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

func (m *clusterSelectionPane) copy() tea.Cmd {
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

// MaybePreviewCluster sends a ClusterDetails message for the currently selected cluster.
func (m *clusterSelectionPane) MaybePreviewCluster(force bool) tea.Cmd {
	if len(m.clusters) == 0 || (m.filtering.enabled && len(m.filtering.matchedClusters) == 0) {
		return func() tea.Msg {
			return messages.ClusterDetails{
				Details: nil,
			}
		}
	}

	idx := m.content.Cursor()
	if m.filtering.enabled && len(m.filtering.matchedClusters) > 0 {
		idx = m.filtering.matchedClusters[idx]
	}
	if idx == m.lastClusterDetails && !force {
		return nil
	}
	m.lastClusterDetails = idx
	cluster := m.clusters[idx]

	return func() tea.Msg {
		return messages.ClusterDetails{
			Details: &cluster,
		}
	}
}

func (m *clusterSelectionPane) Zoom() tea.Cmd {
	return func() tea.Msg {
		return messages.ZoomToggleClusterSelectionPane{}
	}
}

func (m *clusterSelectionPane) selectCluster() tea.Cmd {
	m.cancelDetails()
	rowP := m.content.SelectedRow()
	if rowP == nil {
		return nil
	}
	row := *rowP
	if len(row) == 0 {
		return nil
	}

	// Find the cluster details
	clusterName := row[0].Value()
	var details *apitypes.ClusterItem
	for i := range m.clusters {
		if m.clusters[i].Name == clusterName {
			details = &m.clusters[i]
			break
		}
	}
	if details == nil {
		return nil
	}

	switchView := func() tea.Msg {
		return messages.SwitchView{
			OldView: messages.Cluster_selection,
			NewView: messages.Service_selection,
		}
	}
	selectCluster := func() tea.Msg {
		return messages.SelectCluster{
			ClusterName: clusterName,
			Details:     *details,
		}
	}
	return tea.Batch(switchView, selectCluster)
}

func (m *clusterSelectionPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.updateSize()
}

func (m *clusterSelectionPane) updateSize() {
	h, w := m.window.height, m.window.width

	searchBoxH := u.Ternary(m.search.GetHeight(), 0, m.search.IsEnabled())
	m.content.SetHeight(h - searchBoxH - u.Ternary(1, 0, m.spinner.active))
	m.content.SetWidth(w)
	m.search.SetWidth(w)
}

func (m *clusterSelectionPane) View() string {
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

func (m *clusterSelectionPane) noContentMessage() string {
	if m.spinner.active {
		return ""
	}
	s := strings.Builder{}
	fmt.Fprintf(&s, "==================================================\n")
	fmt.Fprintf(&s, "             NO CLUSTERS FOUND                    \n")
	fmt.Fprintf(&s, "==================================================\n")
	return s.String()
}

func notifyCopySuccess() tea.Msg {
	return messages.ToggleNotificationDialog{Msg: "Copied!", Duration: 1 * time.Second}
}

// openConsole emits an OpenConsole message with the URL to the AWS ECS
// console cluster page for the currently selected cluster.
func (m *clusterSelectionPane) openConsole() tea.Cmd {
	if len(m.clusters) == 0 {
		return nil
	}

	idx := m.content.Cursor()
	if m.filtering.enabled && len(m.filtering.matchedClusters) > 0 {
		idx = m.filtering.matchedClusters[idx]
	}
	if idx >= len(m.clusters) {
		return nil
	}

	cluster := m.clusters[idx]
	region := m.config.Region
	url := fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/home?region=%s#/clusters/%s",
		region, region, cluster.Name)
	return func() tea.Msg {
		return messages.OpenConsole{URL: url}
	}
}

// openHostShell resolves the cluster's EC2 container instances and emits a
// HostShell message directly (single instance) or a
// ContainersResolvedForHostShell message so home.go can show the picker.
func (m *clusterSelectionPane) openHostShell() tea.Cmd {
	if len(m.clusters) == 0 {
		return nil
	}

	idx := m.content.Cursor()
	if m.filtering.enabled && len(m.filtering.matchedClusters) > 0 {
		idx = m.filtering.matchedClusters[idx]
	}
	if idx >= len(m.clusters) {
		return nil
	}

	clusterName := m.clusters[idx].Name
	client := m.config.ECSClient
	region := m.config.Region
	profile := ""
	if m.config.Profile != nil {
		profile = *m.config.Profile
	}
	ctx, cc := context.WithTimeout(m.ctx, m.stdTO)
	_ = cc

	return func() tea.Msg {
		ids, err := ecsadapter.ListClusterInstanceIDs(client, ctx, clusterName)
		if err != nil {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("listing cluster instances: %w", err),
			}
		}
		switch len(ids) {
		case 0:
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("no hosts in cluster %s", clusterName),
			}
		case 1:
			return messages.HostShell{
				EC2InstanceID: ids[0],
				Region:        region,
				Profile:       profile,
			}
		default:
			return messages.ContainersResolvedForHostShell{
				Cluster:   clusterName,
				Instances: ids,
			}
		}
	}
}
