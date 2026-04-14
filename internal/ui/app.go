package ui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"

	ecsaws "github.com/ron/ecsx/internal/aws"
	"github.com/ron/ecsx/internal/logs"
)

type Model struct {
	client  ecsaws.ECSClient
	list    list.Model
	detail  viewport.Model
	spinner spinner.Model

	initialCluster string
	level          viewLevel
	clusterName    string
	serviceName    string

	showEnvVars bool
	envVars     []ecsaws.ContainerDefinition
	showHelp    bool
	scaling     bool
	confirming  bool
	confirmAction string // "scale", "deploy", "stop"
	scaleInput  textinput.Model
	scaleSvc    string
	scaleCount  int32
	metricsMap  map[string]*ecsaws.ServiceMetrics

	// Log tailing
	logLines         []ecsaws.LogEvent
	logCancel        context.CancelFunc
	logCh            <-chan ecsaws.LogEvent
	logFilter        string
	logFiltering     bool
	logFilterInput   textinput.Model
	logGroup         string
	logStreamPrefix  string
	logLevel         logs.Level
	logGrep          string
	logGrepRe        *regexp.Regexp
	logGrepping      bool
	logGrepInput     textinput.Model
	logPinned        bool

	zoomed  bool
	loading bool
	err     error
	width   int
	height  int

	autoRefresh bool
	refreshing  bool // background refresh in progress (no spinner, preserve selection)
}

func New(client ecsaws.ECSClient, cluster string) Model {
	sp := spinner.New()
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Clusters"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	return Model{
		client:         client,
		list:           l,
		spinner:        sp,
		detail:         viewport.New(),
		loading:        true,
		initialCluster: cluster,
		clusterName:    cluster,
		metricsMap:     make(map[string]*ecsaws.ServiceMetrics),
		autoRefresh:    true,
	}
}

const autoRefreshInterval = 30 * time.Second

func autoRefreshCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	if m.initialCluster != "" {
		return tea.Batch(m.spinner.Tick, m.loadServiceNames(m.initialCluster), autoRefreshCmd())
	}
	return tea.Batch(m.spinner.Tick, m.loadClusters(), autoRefreshCmd())
}

// Layout helpers
func (m *Model) listWidth() int   { return m.width * 2 / 5 }
func (m *Model) detailWidth() int { return m.width - m.listWidth() }

func (m *Model) stopLogTail() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	m.logCh = nil
	m.logLines = nil
}

func (m *Model) filteredLogLines() []ecsaws.LogEvent {
	var out []ecsaws.LogEvent
	for _, ev := range m.logLines {
		if !m.logLevel.Matches(ev.Message) {
			continue
		}
		if m.logGrepRe != nil && !m.logGrepRe.MatchString(ev.Message) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func (m *Model) startLogTail() tea.Cmd {
	m.stopLogTail()

	// Pre-fill with historical logs
	m.logLines = []ecsaws.LogEvent{}
	if events, err := m.client.FetchRecentLogs(context.Background(), m.logGroup, m.logStreamPrefix, m.logFilter, time.Now().Add(-5*time.Minute), nil); err == nil {
		m.logLines = events
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.client.TailLogs(ctx, m.logGroup, m.logStreamPrefix, m.logFilter)
	if err != nil {
		cancel()
		m.err = err
		return nil
	}
	m.logCancel = cancel
	m.logCh = ch
	m.level = viewLogs
	m.zoomed = true
	m.logPinned = false
	m.updateDetail()
	m.detail.GotoBottom()
	return m.waitForLogEvent()
}

func (m Model) waitForLogEvent() tea.Cmd {
	if m.logCh == nil {
		return nil
	}
	ch := m.logCh
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return logEventMsg{event: ev}
	}
}

func (m *Model) updateSizes() {
	panelH := m.height - 2
	if panelH < 3 {
		panelH = 3
	}
	innerH := panelH - 2
	m.list.SetSize(m.listWidth()-2, innerH)
	m.detail.SetWidth(m.detailWidth() - 2)
	m.detail.SetHeight(innerH)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		m.updateDetail()
		return m, nil

	case tea.KeyPressMsg:
		if m.scaling {
			switch msg.String() {
			case "enter":
				return m.confirmScale()
			case "esc":
				m.scaling = false
				m.updateDetail()
				return m, nil
			}
			var cmd tea.Cmd
			m.scaleInput, cmd = m.scaleInput.Update(msg)
			m.updateDetail()
			return m, cmd
		}
		if m.confirming {
			switch msg.String() {
			case "enter":
				switch m.confirmAction {
				case "deploy":
					return m.executeForceNewDeployment()
				case "stop":
					return m.executeStopTask()
				default:
					return m.executeScale()
				}
			case "esc":
				m.confirming = false
				m.confirmAction = ""
				m.updateDetail()
				return m, nil
			}
			return m, nil
		}
		if m.logFiltering {
			switch msg.String() {
			case "enter":
				m.logFilter = m.logFilterInput.Value()
				m.logFiltering = false
				return m, m.startLogTail()
			case "esc":
				m.logFiltering = false
				m.updateDetail()
				return m, nil
			}
			var cmd tea.Cmd
			m.logFilterInput, cmd = m.logFilterInput.Update(msg)
			m.updateDetail()
			return m, cmd
		}
		if m.logGrepping {
			switch msg.String() {
			case "enter":
				pattern := m.logGrepInput.Value()
				if pattern == "" {
					m.logGrep = ""
					m.logGrepRe = nil
				} else {
					re, err := regexp.Compile(pattern)
					if err != nil {
						m.err = fmt.Errorf("invalid regex: %w", err)
						m.logGrepping = false
						m.updateDetail()
						return m, nil
					}
					m.logGrep = pattern
					m.logGrepRe = re
				}
				m.logGrepping = false
				m.updateDetail()
				m.detail.GotoBottom()
				return m, nil
			case "esc":
				m.logGrepping = false
				m.updateDetail()
				return m, nil
			}
			var cmd tea.Cmd
			m.logGrepInput, cmd = m.logGrepInput.Update(msg)
			m.updateDetail()
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m, tea.Quit
		case "enter":
			if !m.showHelp && m.list.FilterState() != list.Filtering {
				return m.handleEnter()
			}
		case "esc":
			if m.showHelp {
				m.showHelp = false
				m.updateDetail()
				return m, nil
			}
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m.handleEsc()
		case "e":
			if !m.showHelp && m.list.FilterState() != list.Filtering {
				if m.level == viewLogs && !m.logFiltering {
					return m, openLogsInEditor(m.filteredLogLines())
				}
				return m.handleEnvVars()
			}
		case "s":
			if !m.showHelp && m.list.FilterState() != list.Filtering && m.level == viewServices {
				return m.handleScale()
			}
		case "?":
			if m.list.FilterState() != list.Filtering {
				m.showHelp = !m.showHelp
				if m.showHelp {
					m.detail.SetContent(m.renderHelp())
				} else {
					m.updateDetail()
				}
				return m, nil
			}
		case "r":
			if m.list.FilterState() != list.Filtering && !m.showHelp {
				return m.handleRefresh()
			}
		case "a":
			if m.list.FilterState() != list.Filtering && !m.showHelp {
				m.autoRefresh = !m.autoRefresh
				return m, nil
			}
		case "y":
			if m.list.FilterState() != list.Filtering && !m.showHelp && m.showEnvVars {
				return m.handleYank()
			}
		case "l":
			if m.list.FilterState() != list.Filtering && !m.showHelp && (m.level == viewServices || m.level == viewTasks) {
				return m.handleLogs()
			}
		case "x":
			if m.list.FilterState() != list.Filtering && !m.showHelp {
				switch m.level {
				case viewServices:
					return m.handleForceNewDeployment()
				case viewTasks:
					return m.handleStopTask()
				case viewClusters:
					return m.handleSSM()
				}
			}
		case "+", "-":
			if m.list.FilterState() != list.Filtering && !m.showHelp {
				m.zoomed = !m.zoomed
				m.updateSizes()
				m.updateDetail()
				return m, nil
			}
		case "up", "k", "pgup", "home":
			if m.level == viewLogs && !m.showHelp && !m.logFiltering && !m.logGrepping {
				m.logPinned = true
				var cmd tea.Cmd
				m.detail, cmd = m.detail.Update(msg)
				return m, cmd
			}
		case "down", "j", "pgdown":
			if m.level == viewLogs && !m.showHelp && !m.logFiltering && !m.logGrepping {
				var cmd tea.Cmd
				m.detail, cmd = m.detail.Update(msg)
				m.logPinned = !m.detail.AtBottom()
				return m, cmd
			}
		case "end":
			if m.level == viewLogs && !m.showHelp && !m.logFiltering && !m.logGrepping {
				m.logPinned = false
				m.detail.GotoBottom()
				return m, nil
			}
		case "f":
			if m.level == viewLogs && !m.showHelp && !m.logFiltering {
				m.logLevel = m.logLevel.Toggle()
				m.updateDetail()
				m.detail.GotoBottom()
				return m, nil
			}
		case "g":
			if m.level == viewLogs && !m.showHelp && !m.logFiltering {
				m.logGrepping = true
				ti := textinput.New()
				ti.Placeholder = "regex pattern"
				ti.SetValue(m.logGrep)
				ti.SetWidth(40)
				m.logGrepInput = ti
				cmd := m.logGrepInput.Focus()
				m.updateDetail()
				m.detail.GotoTop()
				return m, cmd
			}
		case "/":
			if m.level == viewLogs && !m.showHelp {
				m.logFiltering = true
				ti := textinput.New()
				ti.Placeholder = "filter pattern"
				ti.SetValue(m.logFilter)
				ti.SetWidth(40)
				m.logFilterInput = ti
				cmd := m.logFilterInput.Focus()
				m.updateDetail()
				m.detail.GotoTop()
				return m, cmd
			}
		}

	case clustersLoadedMsg:
		m.loading = false
		m.level = viewClusters
		if !m.refreshing {
			m.list.ResetFilter()
		}
		prevSel := ""
		if sel := m.list.SelectedItem(); sel != nil {
			prevSel = sel.FilterValue()
		}
		items := make([]list.Item, len(msg.clusters))
		for i, c := range msg.clusters {
			items[i] = clusterItem{c}
		}
		cmd := m.list.SetItems(items)
		m.list.Title = "Clusters"
		if m.refreshing {
			m.refreshing = false
			for i, item := range items {
				if item.FilterValue() == prevSel {
					m.list.Select(i)
					break
				}
			}
		} else {
			m.list.Select(0)
		}
		m.updateSizes()
		m.updateDetail()
		return m, cmd

	case serviceNamesLoadedMsg:
		m.loading = false
		m.level = viewServices
		if !m.refreshing {
			m.list.ResetFilter()
			m.metricsMap = make(map[string]*ecsaws.ServiceMetrics)
		}
		prevSel := ""
		if sel := m.list.SelectedItem(); sel != nil {
			prevSel = sel.FilterValue()
		}
		items := make([]list.Item, len(msg.names))
		for i, name := range msg.names {
			items[i] = serviceItem{name: name}
		}
		cmd := m.list.SetItems(items)
		m.list.Title = fmt.Sprintf("Services (%s)", m.clusterName)
		if m.refreshing {
			m.refreshing = false
			for i, item := range items {
				if item.FilterValue() == prevSel {
					m.list.Select(i)
					break
				}
			}
		} else {
			m.list.Select(0)
		}
		m.updateSizes()
		m.updateDetail()
		if len(msg.names) > 0 {
			sel := m.list.SelectedItem()
			if sel != nil {
				return m, tea.Batch(cmd, m.loadServiceDetail(m.clusterName, sel.FilterValue()))
			}
		}
		return m, cmd

	case serviceDetailLoadedMsg:
		items := m.list.Items()
		for i, item := range items {
			if si, ok := item.(serviceItem); ok && si.name == msg.name {
				items[i] = serviceItem{name: msg.name, service: msg.service}
				break
			}
		}
		cmd := m.list.SetItems(items)
		if sel := m.list.SelectedItem(); sel != nil {
			if si, ok := sel.(serviceItem); ok && si.name == msg.name {
				m.updateDetail()
				if _, ok := m.metricsMap[msg.name]; !ok {
					return m, tea.Batch(cmd, m.loadMetrics(m.clusterName, msg.name))
				}
			}
		}
		return m, cmd

	case servicesLoadedMsg:
		m.loading = false
		m.level = viewServices
		m.list.ResetFilter()
		m.metricsMap = make(map[string]*ecsaws.ServiceMetrics)
		items := make([]list.Item, len(msg.services))
		for i, s := range msg.services {
			s := s
			items[i] = serviceItem{name: s.Name, service: &s}
		}
		cmd := m.list.SetItems(items)
		m.list.Title = fmt.Sprintf("Services (%s)", m.clusterName)
		m.list.Select(0)
		m.updateSizes()
		m.updateDetail()
		if len(msg.services) > 0 {
			return m, tea.Batch(cmd, m.loadMetrics(m.clusterName, msg.services[0].Name))
		}
		return m, cmd

	case tasksLoadedMsg:
		m.loading = false
		m.level = viewTasks
		if !m.refreshing {
			m.list.ResetFilter()
		}
		prevSel := ""
		if sel := m.list.SelectedItem(); sel != nil {
			prevSel = sel.FilterValue()
		}
		items := make([]list.Item, len(msg.tasks))
		for i, t := range msg.tasks {
			items[i] = taskItem{t}
		}
		cmd := m.list.SetItems(items)
		m.list.Title = fmt.Sprintf("Tasks (%s)", m.serviceName)
		if m.refreshing {
			m.refreshing = false
			for i, item := range items {
				if item.FilterValue() == prevSel {
					m.list.Select(i)
					break
				}
			}
		} else {
			m.list.Select(0)
		}
		m.updateSizes()
		m.updateDetail()
		return m, tea.Batch(cmd, m.resolveTaskEC2(m.clusterName, msg.tasks))

	case taskEC2ResolvedMsg:
		if msg.ec2Map == nil || m.level != viewTasks {
			return m, nil
		}
		items := m.list.Items()
		for i, item := range items {
			if ti, ok := item.(taskItem); ok {
				if id, found := msg.ec2Map[ti.task.ContainerInstanceID]; found {
					ti.task.EC2InstanceID = id
					items[i] = ti
				}
			}
		}
		m.list.SetItems(items)
		m.updateDetail()
		return m, nil

	case envVarsLoadedMsg:
		m.loading = false
		m.showEnvVars = true
		m.envVars = msg.containers
		m.updateDetail()
		return m, nil

	case serviceScaledMsg:
		m.loading = false
		return m, m.loadServiceNames(m.clusterName)

	case serviceRedeployedMsg:
		m.loading = false
		return m, m.loadServiceNames(m.clusterName)

	case taskStoppedMsg:
		m.loading = false
		return m, m.loadTasks(m.clusterName, m.serviceName)

	case metricsLoadedMsg:
		m.metricsMap[msg.service] = msg.metrics
		if m.level == viewServices {
			m.updateDetail()
		}
		return m, nil

	case logEventMsg:
		m.logLines = append(m.logLines, msg.event)
		if len(m.logLines) > 1000 {
			m.logLines = m.logLines[len(m.logLines)-1000:]
		}
		m.updateDetail()
		if !m.logFiltering && !m.logGrepping && !m.logPinned {
			m.detail.GotoBottom()
		}
		return m, m.waitForLogEvent()

	case logErrorMsg:
		m.err = msg.err
		return m, nil

	case logGroupFoundMsg:
		m.loading = false
		m.logGroup = msg.logGroup
		m.logStreamPrefix = msg.streamPrefix
		return m, m.startLogTail()

	case ec2InstancesLoadedMsg:
		m.loading = false
		if len(msg.instances) == 0 {
			m.err = fmt.Errorf("no container instances in cluster %s", m.clusterName)
			return m, nil
		}
		m.level = viewEC2Instances
		if !m.refreshing {
			m.list.ResetFilter()
		}
		prevSel := ""
		if sel := m.list.SelectedItem(); sel != nil {
			prevSel = sel.FilterValue()
		}
		items := make([]list.Item, len(msg.instances))
		for i, inst := range msg.instances {
			items[i] = ec2Item{inst}
		}
		cmd := m.list.SetItems(items)
		m.list.Title = fmt.Sprintf("EC2 Instances (%s)", m.clusterName)
		if m.refreshing {
			m.refreshing = false
			for i, item := range items {
				if item.FilterValue() == prevSel {
					m.list.Select(i)
					break
				}
			}
		} else {
			m.list.Select(0)
		}
		m.updateSizes()
		m.updateDetail()
		return m, cmd

	case ssmTargetMsg:
		m.loading = false
		return m, startSSMSession(msg.instanceID, m.client.Region(), m.client.Profile())

	case ssmFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case editorFinishedMsg:
		if msg.path != "" {
			os.Remove(msg.path)
		}
		if msg.err != nil {
			m.err = msg.err
		}
		m.updateDetail()
		return m, nil

	case autoRefreshMsg:
		// Always re-schedule the next tick
		next := autoRefreshCmd()
		if !m.autoRefresh || m.loading || m.err != nil || m.scaling || m.confirming || m.showHelp || m.logFiltering || m.logGrepping || m.list.FilterState() == list.Filtering {
			return m, next
		}
		if cc, ok := m.client.(*ecsaws.CachedClient); ok {
			cc.Purge()
		}
		m.refreshing = true
		switch m.level {
		case viewClusters:
			return m, tea.Batch(next, m.loadClusters())
		case viewServices:
			return m, tea.Batch(next, m.loadServiceNames(m.clusterName))
		case viewTasks:
			return m, tea.Batch(next, m.loadTasks(m.clusterName, m.serviceName))
		case viewEC2Instances:
			return m, tea.Batch(next, m.loadContainerInstances(m.clusterName))
		}
		return m, next

	case errMsg:
		m.loading = false
		if m.initialCluster != "" && m.level == viewClusters {
			m.initialCluster = ""
			m.clusterName = ""
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadClusters())
		}
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if !m.loading && m.err == nil && !m.showHelp && !m.scaling && !m.confirming {
		var prevName string
		if sel := m.list.SelectedItem(); sel != nil {
			prevName = sel.FilterValue()
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		var curName string
		if sel := m.list.SelectedItem(); sel != nil {
			curName = sel.FilterValue()
		}
		if curName != prevName {
			m.updateDetail()
			if m.level == viewServices && curName != "" {
				si := m.list.SelectedItem().(serviceItem)
				if si.service == nil {
					return m, tea.Batch(cmd, m.loadServiceDetail(m.clusterName, si.name))
				}
				if _, ok := m.metricsMap[si.name]; !ok {
					return m, tea.Batch(cmd, m.loadMetrics(m.clusterName, si.name))
				}
			}
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) View() tea.View {
	var content string

	if m.err != nil {
		errText := errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
		hint := helpStyle.Render("\n\n  Press esc to go back, q to quit")
		content = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top,
			fmt.Sprintf("\n%s%s", errText, hint))
	} else if m.loading {
		content = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Center,
			fmt.Sprintf("  %s Loading...", m.spinner.View()))
	} else {
		panelH := m.height - 2
		if panelH < 3 {
			panelH = 3
		}
		innerH := panelH - 2
		bc := "  " + m.breadcrumb()
		help := helpStyle.Render(m.helpText())

		if m.zoomed {
			fw := m.width
			m.detail.SetWidth(fw - 2)
			m.detail.SetHeight(innerH)
			rightContent := lipgloss.Place(fw-2, innerH, lipgloss.Left, lipgloss.Top, m.detail.View())
			pane := paneBorder.Render(rightContent)
			content = lipgloss.JoinVertical(lipgloss.Left, bc, pane, help)
		} else {
			lw := m.listWidth()
			dw := m.detailWidth()

			m.list.SetSize(lw-2, innerH)
			m.detail.SetWidth(dw - 2)
			m.detail.SetHeight(innerH)

			leftContent := lipgloss.Place(lw-2, innerH, lipgloss.Left, lipgloss.Top, m.list.View())
			rightContent := lipgloss.Place(dw-2, innerH, lipgloss.Left, lipgloss.Top, m.detail.View())

			leftPane := activeBorder.Render(leftContent)
			rightPane := paneBorder.Render(rightContent)

			panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
			content = lipgloss.JoinVertical(lipgloss.Left, bc, panes, help)
		}
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "ecsx"
	return v
}
