package ui

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appconfig "github.com/ron/ecsx/pkg"
	"github.com/ron/ecsx/pkg/ui/internal/dialogs"
	"github.com/ron/ecsx/pkg/ui/internal/messages"
	commonstyles "github.com/ron/ecsx/pkg/ui/internal/styles"
	"github.com/ron/ecsx/pkg/ui/internal/views/clusters"
	clustersView "github.com/ron/ecsx/pkg/ui/internal/views/clusters"
	"github.com/ron/ecsx/pkg/ui/internal/views/services"
	"github.com/ron/ecsx/pkg/ui/internal/views/tasks"
)

type View int
type Dialog int

const (
	tables_view View = iota
	items_view
)

const (
	help_dialog Dialog = iota
	regions_dialog
	mfa_dialog
)

var regionBlock = lipgloss.NewStyle().
	Background(commonstyles.RegionBoxBg).
	Align(lipgloss.Left, lipgloss.Top).
	PaddingLeft(1).
	PaddingRight(1).
	Height(1)

type Model struct {
	// ActiveView determines tea.Msg forwarding
	activeView View

	KeyMap KeyMap

	window struct {
		width  int
		height int
	}

	// dialogs
	dialogs struct {
		open   bool
		help   *dialogs.Help
		region *dialogs.Regions
		mfa    *dialogs.MFA
		active Dialog

		notification []*dialogs.NotificationDialog
	}

	// top-level context
	ctx context.Context

	// shared config
	config *appconfig.Config

	// views
	clusterSelection *clustersView.ItemSelection
	// itemselection  *itemsview.ItemSelection

	// help
	Help help.Model

	// additional options
	options options
}

type options struct {
	InitialError error
}

type Option func(*options)

func WithInitialErrorNotification(err error) Option {
	return func(o *options) {
		o.InitialError = err
	}
}

func NewModel(ctx context.Context, cfg appconfig.Config, client *ecs.Client, opts ...Option) Model {
	m := Model{
		ctx:         ctx,
		cfg:         cfg,
		client:      client,
		activeView:  messages.ClusterSelection,
		clusters:    clusters.NewView(ctx, client),
		services:    services.NewView(ctx, client),
		tasks:       tasks.NewView(ctx, client),
		initCluster: cfg.Cluster,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	// If a cluster was provided via flag, skip straight to services.
	if m.initCluster != "" {
		m.activeView = messages.ServiceSelection
		return m.services.Load(m.initCluster)
	}
	return m.clusters.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.window.width = msg.Width
		m.window.height = msg.Height
		return m, m.forward(msg)

	case messages.SelectCluster:
		m.activeView = messages.ServiceSelection
		return m, m.services.Load(msg.ClusterName)

	case services.BackToClustersMsg:
		m.activeView = messages.ClusterSelection
		return m, nil

	case services.SelectServiceMsg:
		m.activeView = messages.TaskSelection
		return m, m.tasks.Load(msg.Cluster, msg.ServiceName)

	case tasks.BackToServicesMsg:
		m.activeView = messages.ServiceSelection
		return m, m.services.Load(msg.Cluster)
	}

	return m, m.forward(msg)
}

func (m Model) forward(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.WindowSizeMsg:
		return m.broadcast(msg)
	default:
		return m.routeToActiveOnly(msg)
	}
}

func (m Model) routeToActiveOnly(msg tea.Msg) tea.Cmd {
	switch m.activeView {
	case messages.ClusterSelection:
		return m.clusters.Update(msg)
	case messages.ServiceSelection:
		return m.services.Update(msg)
	case messages.TaskSelection:
		return m.tasks.Update(msg)
	}
	return nil
}

func (m Model) broadcast(msg tea.Msg) tea.Cmd {
	return tea.Batch(
		m.clusters.Update(msg),
		m.services.Update(msg),
		m.tasks.Update(msg),
	)
}

func (m Model) View() tea.View {
	var page string
	switch m.activeView {
	case messages.ClusterSelection:
		page = m.clusters.View()
	case messages.ServiceSelection:
		page = m.services.View()
	case messages.TaskSelection:
		page = m.tasks.View()
	}
	v := tea.NewView(page)
	v.AltScreen = true
	return v
}
