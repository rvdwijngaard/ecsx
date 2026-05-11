package ui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	appconfig "github.com/ron/ecsx/pkg"
	"github.com/ron/ecsx/pkg/ui/internal/messages"
	"github.com/ron/ecsx/pkg/ui/internal/views/clusters"
	"github.com/ron/ecsx/pkg/ui/internal/views/services"
	"github.com/ron/ecsx/pkg/ui/internal/views/tasks"
)

type Model struct {
	ctx    context.Context
	cfg    appconfig.Config
	client *ecs.Client

	activeView messages.View

	KeyMap KeyMap

	window struct {
		width  int
		height int
	}

	// views
	clusters *clusters.View
	services *services.View
	tasks    *tasks.View

	// if set, skip cluster selection and go straight to services
	initCluster string
}

type Option func(*Model)

func WithInitialCluster(cluster string) Option {
	return func(m *Model) {
		m.initCluster = cluster
	}
}

func NewModel(ctx context.Context, cfg appconfig.Config, client *ecs.Client, opts ...Option) Model {
	m := Model{
		ctx:        ctx,
		cfg:        cfg,
		client:     client,
		activeView: messages.ClusterSelection,
		KeyMap:     DefaultKeyMap(),
		clusters:   clusters.NewView(ctx, client),
		services:   services.NewView(ctx, client),
		tasks:      tasks.NewView(ctx, client),
	}
	for _, opt := range opts {
		opt(&m)
	}
	if m.cfg.Cluster != "" {
		m.initCluster = m.cfg.Cluster
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if m.initCluster != "" {
		m.activeView = messages.ServiceSelection
		return m.services.Load(m.initCluster)
	}
	return m.clusters.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, m.KeyMap.ForceQuit) {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.window.width = msg.Width
		m.window.height = msg.Height
		return m, m.broadcast(msg)

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

	return m, m.routeToActive(msg)
}

func (m Model) routeToActive(msg tea.Msg) tea.Cmd {
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
