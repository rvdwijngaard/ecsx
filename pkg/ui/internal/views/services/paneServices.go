package serviceselection

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

// previewTickMsg is used to debounce service preview updates.
type previewTickMsg struct {
	seq int
}

type ServiceTableStyles struct {
	SelectedBackground    color.Color
	SearchMatchBackground color.Color
}

type serviceSelectionPane struct {
	// top-level context
	ctx context.Context

	// shared config (ECS client accessed via config.ECSClient)
	config *appconfig.Config

	// styles
	styles struct {
		Table ServiceTableStyles
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

	// cancel loading services
	cancelServices func()

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
	KeyMap *ServicePaneKeyMap

	// Additional Keys
	AddKeyMap keymaps.AdditionalKeys

	// the underlying table component
	content *table.Model

	// the services retrieved from ECS
	services []apitypes.ServiceItem

	// the currently selected cluster
	clusterName string

	// filtering parameters
	filtering struct {
		matchedServices []int   // indices referring to services
		matchedRunes    [][]int // matches by index to filtering.matchedServices
		enabled         bool
	}

	// index to most recently previewed service
	lastServiceDetails int

	// debounce sequence for preview
	previewSeq int
}

type servicePaneOption func(p *serviceSelectionPane)

func withServicePaneKeys(keys keymaps.AdditionalKeys) servicePaneOption {
	return func(t *serviceSelectionPane) {
		t.AddKeyMap = keys
	}
}

func newServiceSelectionPane(ctx context.Context, config *appconfig.Config, opts ...servicePaneOption) *serviceSelectionPane {
	p := &serviceSelectionPane{
		ctx:            ctx,
		config:         config,
		cancelDetails:  func() {}, // noop on init
		cancelServices: func() {}, // noop on init
		debounceDur:    50 * time.Millisecond,
		stdTO:          30 * time.Second,
		KeyMap:         DefaultServicePaneKeyMap(),
	}

	{ // contents table
		t := table.New(
			table.WithColumns([]table.Column{{Title: "service-name", Width: 64}}),
			table.WithFocused(true),
			table.WithFieldDelegate(p.ServiceRowFieldDelegate),
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

		st := ServiceTableStyles{
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
		p.spinner.text = "loading services..."
	}

	{ // search box
		p.search = search.NewSearchBox(
			search.SearchCallbacks{
				ToSearch: func(string) []string {
					return table.Rows(p.content.Rows()).ToStrings()
				},
				EmptyInput: func() tea.Cmd {
					p.filtering.enabled = false
					p.filtering.matchedServices = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					return p.MaybePreviewService(true)
				},
				Results: func(_ string, results []search.FilteredItem) tea.Cmd {
					p.filtering.enabled = true
					p.filtering.matchedServices = make([]int, len(results))
					p.filtering.matchedRunes = make([][]int, len(results))
					rows := p.content.Rows()
					filtered := make([]table.Row, len(results))
					for i, match := range results {
						filtered[i] = rows[match.Index]
						p.filtering.matchedServices[i] = match.Index
						p.filtering.matchedRunes[i] = match.Matches
					}
					p.content.SetVirtualRows(filtered)
					return nil
				},
				Reset: func(searchHeight int) tea.Cmd {
					p.filtering.enabled = false
					p.filtering.matchedServices = make([]int, 0)
					p.filtering.matchedRunes = make([][]int, 0)
					p.content.ResetVirtualRows()
					p.updateSize()
					return p.MaybePreviewService(true)
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

func (m *serviceSelectionPane) cleanSlate() {
	m.err = nil
}

func (m *serviceSelectionPane) Init() tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.services = []apitypes.ServiceItem{}
	m.clusterName = ""

	// cancel any lingering calls
	m.cancelDetails()
	m.cancelServices()
	return nil
}

func (m *serviceSelectionPane) loadServices() tea.Cmd {
	spinnerCmd := m.activateSpinner()

	client := m.config.ECSClient
	cluster := m.clusterName
	ctx, cc := context.WithTimeout(m.ctx, m.stdTO)
	m.cancelServices = cc

	return tea.Batch(func() tea.Msg {
		defer cc()
		services, err := ecsadapter.ListServices(client, ctx, cluster)
		return messages.ServicePageReady{
			Cluster:  cluster,
			Services: services,
			Err:      err,
		}
	}, spinnerCmd)
}

func (m *serviceSelectionPane) activateSpinner() tea.Cmd {
	m.spinner.active = true
	m.updateSize()
	return m.spinner.model.Tick
}

func (m *serviceSelectionPane) deactivateSpinner() {
	m.spinner.active = false
	m.updateSize()
}

func (m *serviceSelectionPane) processServicePage(msg messages.ServicePageReady) tea.Cmd {
	m.deactivateSpinner()
	if msg.Cluster != m.clusterName { // expired
		return nil
	}
	if msg.Err != nil {
		m.err = msg.Err
		return nil
	}
	m.services = msg.Services

	rows := make([]table.Row, len(m.services))
	for i, s := range m.services {
		rows[i] = []table.Field{
			enrichedField{value: s.Name},
		}
	}
	m.content.SetRows(rows)

	return m.MaybePreviewService(true)
}

func (m *serviceSelectionPane) Update(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case messages.ServiceDetails:
		return nil
	case messages.TaskDetails:
		return nil
	case messages.TaskPageReady:
		return nil
	case messages.ClusterDetails:
		return nil
	case messages.ClusterPageReady:
		return nil
	case messages.SelectCluster:
		return m.selectCluster(msg.ClusterName)
	case messages.ServicePageReady:
		return m.processServicePage(msg)
	case previewTickMsg:
		if msg.seq == m.previewSeq {
			return m.emitPreview()
		}
		return nil
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

	cmds = append(cmds, m.MaybePreviewService(false))
	return tea.Batch(cmds...)
}

func (m *serviceSelectionPane) selectCluster(clusterName string) tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.services = []apitypes.ServiceItem{}
	m.clusterName = clusterName
	m.lastServiceDetails = -1

	m.cancelDetails()
	m.cancelServices()
	return m.loadServices()
}

func (m *serviceSelectionPane) selectService() tea.Cmd {
	m.cancelDetails()
	m.cancelServices()
	rowP := m.content.SelectedRow()
	if rowP == nil {
		return nil
	}
	row := *rowP
	if len(row) == 0 {
		return nil
	}

	serviceName := row[0].Value()
	var details *apitypes.ServiceItem
	for i := range m.services {
		if m.services[i].Name == serviceName {
			details = &m.services[i]
			break
		}
	}
	if details == nil {
		return nil
	}

	switchView := func() tea.Msg {
		return messages.SwitchView{
			OldView: messages.Service_selection,
			NewView: messages.Task_selection,
		}
	}
	selectService := func() tea.Msg {
		return messages.SelectService{
			ClusterName: m.clusterName,
			ServiceName: serviceName,
			Details:     *details,
		}
	}
	return tea.Batch(switchView, selectService)
}

// handleNavigation handles events when search is not active.
func (m *serviceSelectionPane) handleNavigation(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Search):
			cmds = append(cmds, m.search.OpenSearchBox())
		case key.Matches(msg, m.KeyMap.Select):
			return m.selectService()
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
		case key.Matches(msg, m.KeyMap.Deploy):
			return m.forceNewDeployment()
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

func (m *serviceSelectionPane) ServiceRowFieldDelegate(row table.Row, col table.Column, colIdx, rowIdx, colW, padL, padR int, selected bool) string {
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

func (m *serviceSelectionPane) copy() tea.Cmd {
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

// forceNewDeployment emits a ForceNewDeployment message for the currently selected service.
func (m *serviceSelectionPane) forceNewDeployment() tea.Cmd {
	if len(m.services) == 0 {
		return nil
	}

	idx := m.content.Cursor()
	if m.filtering.enabled && len(m.filtering.matchedServices) > 0 {
		idx = m.filtering.matchedServices[idx]
	}
	if idx >= len(m.services) {
		return nil
	}

	service := m.services[idx]
	cluster := m.clusterName
	return func() tea.Msg {
		return messages.ForceNewDeployment{
			Cluster: cluster,
			Service: service.Name,
		}
	}
}

// MaybePreviewService schedules a debounced preview update for the currently selected service.
func (m *serviceSelectionPane) MaybePreviewService(force bool) tea.Cmd {
	if len(m.services) == 0 || (m.filtering.enabled && len(m.filtering.matchedServices) == 0) {
		if m.lastServiceDetails == -1 && !force {
			return nil
		}
		m.lastServiceDetails = -1
		return func() tea.Msg {
			return messages.ServiceDetails{
				Details: nil,
			}
		}
	}

	idx := m.content.Cursor()
	if m.filtering.enabled && len(m.filtering.matchedServices) > 0 {
		idx = m.filtering.matchedServices[idx]
	}
	if idx == m.lastServiceDetails && !force {
		return nil
	}
	m.lastServiceDetails = idx

	if force {
		// Immediate emit (e.g. after loading services or resetting search)
		return m.emitPreview()
	}

	// Debounced: increment sequence and schedule a tick
	m.previewSeq++
	seq := m.previewSeq
	dur := m.debounceDur
	return tea.Tick(dur, func(_ time.Time) tea.Msg {
		return previewTickMsg{seq: seq}
	})
}

// emitPreview sends the ServiceDetails message for the current lastServiceDetails index.
func (m *serviceSelectionPane) emitPreview() tea.Cmd {
	if m.lastServiceDetails < 0 || m.lastServiceDetails >= len(m.services) {
		return func() tea.Msg {
			return messages.ServiceDetails{Details: nil}
		}
	}
	service := m.services[m.lastServiceDetails]
	return func() tea.Msg {
		return messages.ServiceDetails{
			Details: &service,
		}
	}
}

func (m *serviceSelectionPane) Zoom() tea.Cmd {
	return func() tea.Msg {
		return messages.ZoomToggleServiceSelectionPane{}
	}
}

func (m *serviceSelectionPane) escape() tea.Cmd {
	m.cancelDetails()
	m.cancelServices()

	switchView := func() tea.Msg {
		return messages.SwitchView{
			OldView: messages.Service_selection,
			NewView: messages.Cluster_selection,
		}
	}
	resetPreview := func() tea.Msg {
		return messages.ServiceDetails{Details: nil}
	}
	return tea.Batch(switchView, resetPreview)
}

func (m *serviceSelectionPane) reload() tea.Cmd {
	m.search.Reset()
	m.content.ResetVirtualRows()
	m.content.SetCursor(0)
	m.cleanSlate()
	m.services = []apitypes.ServiceItem{}
	m.lastServiceDetails = -1

	m.cancelDetails()
	m.cancelServices()
	return m.loadServices()
}

func (m *serviceSelectionPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	m.updateSize()
}

func (m *serviceSelectionPane) updateSize() {
	h, w := m.window.height, m.window.width

	searchBoxH := u.Ternary(m.search.GetHeight(), 0, m.search.IsEnabled())
	m.content.SetHeight(h - searchBoxH - u.Ternary(1, 0, m.spinner.active))
	m.content.SetWidth(w)
	m.search.SetWidth(w)
}

func (m *serviceSelectionPane) View() string {
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

func (m *serviceSelectionPane) noContentMessage() string {
	if m.spinner.active {
		return ""
	}
	s := strings.Builder{}
	fmt.Fprintf(&s, "==================================================\n")
	fmt.Fprintf(&s, "          NO SERVICES IN THIS CLUSTER             \n")
	fmt.Fprintf(&s, "==================================================\n")
	return s.String()
}

func notifyCopySuccess() tea.Msg {
	return messages.ToggleNotificationDialog{Msg: "Copied!", Duration: 1 * time.Second}
}
