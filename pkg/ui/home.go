package ui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"

	appconfig "github.com/rvdwijngaard/ecsx/pkg"
	"github.com/rvdwijngaard/ecsx/pkg/aws"
	cwconnector "github.com/rvdwijngaard/ecsx/pkg/aws/cloudwatch"
	cwlconnector "github.com/rvdwijngaard/ecsx/pkg/aws/cloudwatchlogs"
	ecsconnector "github.com/rvdwijngaard/ecsx/pkg/aws/ecs"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/dialogs"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/messages"
	cwladapter "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs"
	envvarsadapter "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/envvars"
	commonstyles "github.com/rvdwijngaard/ecsx/pkg/ui/internal/styles"
	clustersview "github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/clusters"
	logsview "github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/logs"
	envvarsview "github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/envvars"
	servicesview "github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/services"
	tasksview "github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/tasks"
	"github.com/rvdwijngaard/ecsx/pkg/ui/internal/views/util/keymaps"
	u "github.com/rvdwijngaard/ecsx/pkg/util"
)

type View int
type Dialog int

const (
	clusters_view View = iota
	services_view
	tasks_view
	logs_view
	envvars_view
)

const (
	help_dialog Dialog = iota
	regions_dialog
	mfa_dialog
	container_picker_dialog
	logs_command_dialog
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
		open            bool
		help            *dialogs.Help
		region          *dialogs.Regions
		mfa             *dialogs.MFA
		containerPicker *dialogs.ContainerPicker
		logsCommand     *dialogs.LogsCommandEditor
		active          Dialog

		notification []*dialogs.NotificationDialog
	}

	// top-level context
	ctx context.Context

	// shared config
	config *appconfig.Config

	// views
	clusterSelection *clustersview.ClusterSelection
	serviceSelection *servicesview.ServiceSelection
	taskSelection    *tasksview.TaskSelection
	logsView         *logsview.LogsView
	envVarsView      *envvarsview.EnvVarsView

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

func NewModel(ctx context.Context, cfg appconfig.Config, opts ...Option) Model {
	m := Model{
		ctx:    ctx,
		config: &cfg,

		activeView: clusters_view,
		Help:       help.New(),
	}

	for _, o := range opts {
		o(&m.options)
	}

	m.KeyMap = DefaultKeyMap()

	inheritedKeys := []keymaps.AdditionalKey{
		{
			Binding: m.KeyMap.ForceQuit,
			Call:    tea.Quit,
		}, {
			Binding: m.KeyMap.Help,
			Call:    m.SignalOpenHelpDialog(),
		},
	}

	{ // mfa dialog
		m.dialogs.mfa = dialogs.NewMFADialog(cfg.MFACredentialC)
	}

	{ // container picker dialog
		m.dialogs.containerPicker = dialogs.NewContainerPicker()
	}

	{ // logs command editor dialog
		m.dialogs.logsCommand = dialogs.NewLogsCommandEditor()
	}

	{ // views
		m.clusterSelection = clustersview.NewClusterSelectionView(ctx, &cfg, clustersview.WithAdditionalKeys(keymaps.AdditionalKeys(inheritedKeys)))
		m.serviceSelection = servicesview.NewServiceSelectionView(ctx, &cfg, servicesview.WithAdditionalKeys(keymaps.AdditionalKeys(inheritedKeys)))
		m.taskSelection = tasksview.NewTaskSelectionView(ctx, &cfg, tasksview.WithAdditionalKeys(keymaps.AdditionalKeys(inheritedKeys)))
		m.logsView = logsview.NewLogsView(&cfg, logsview.WithAdditionalKeys(keymaps.AdditionalKeys(inheritedKeys)))
		m.envVarsView = envvarsview.NewEnvVarsView(&cfg)
	}

	{ // cluster view bound dialogs
		clusterViewDialogKeys := m.clusterSelection.DialogKeyMaps()
		m.dialogs.help = dialogs.NewHelp(m.serviceSelection, m.taskSelection, DialogCloseKeymapFrom(m.KeyMap.Help))
		m.dialogs.region = dialogs.NewRegionsDialog(m.config.AvailableRegions, m.config.StarredRegions, m.config.Region, DialogCloseKeymapFrom(clusterViewDialogKeys.RegionDialog))
	}

	return m
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// notify user of any initialisation errors
	if err := m.options.InitialError; err != nil {
		cmds = append(cmds, errorMsg("", err))
		m.options.InitialError = nil // reset
	}

	// load a new aws client
	cfg, err := aws.LoadAWSConfig(m.ctx, m.config.Region, m.config.Profile, m.config.MFACredentialCB)
	if err != nil {
		cmds = append(cmds, errorMsg("", err))
		return tea.Batch(cmds...)
	}

	// set and reinitialise
	m.config.ECSClient = ecsconnector.NewClient(cfg, u.IfNotNil(m.config.Profile, "")).ECS
	m.config.CloudWatchLogsClient = cwlconnector.NewClient(cfg).Logs
	m.config.CloudWatchClient = cwconnector.NewClient(cfg).CW
	cmds = append(cmds, m.clusterSelection.Init())
	cmds = append(cmds, m.serviceSelection.Init())
	cmds = append(cmds, m.taskSelection.Init())

	return tea.Batch(cmds...)
}

// errorMsg returns a command to open a new error notification dialog
func errorMsg(msg string, err error) tea.Cmd {
	return func() tea.Msg {
		return messages.ToggleNotificationDialog{Msg: msg, Error: err}
	}
}

// update handles the message and proceeds to forward it to the model's children
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(msg, m.KeyMap.ForceQuit) {
		return m, tea.Quit
	}
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case appconfig.CredentialsRequest:
		m, cmd = m.OpenMFADialog()
	case messages.CloseMFADialog:
		m, cmd = m.CloseMFADialog()
	case messages.SwitchView:
		m, cmd = m.handleSwitchView(msg)
	case tea.WindowSizeMsg:
		m = m.applySize(msg.Height, msg.Width).(Model)
	case messages.ToggleHelp:
		m, cmd = m.ToggleHelpDialog()
	case messages.ToggleRegions:
		m, cmd = m.ToggleRegionsDialog()
	case messages.ToggleNotificationDialog:
		m, cmd = m.ToggleNotificationDialog(msg)
	case messages.NotificationExpired:
		m, cmd = m.HandleExpiredError(msg)
	case messages.NotificationTick:
		m, cmd = m.handleErrorTick(msg)
	case messages.SwitchRegion:
		m, cmd = m.switchRegion(msg.OldRegion, msg.NewRegion)
	case messages.OpenLogs:
		if msg.Container != "" {
			// Container selected — use external viewer or internal logs view
			if m.config.LogsViewer != "" {
				cmd = logsview.OpenInExternalViewer(
					m.config.LogsViewer,
					m.config.ECSClient,
					m.config.CloudWatchLogsClient,
					msg.Cluster, msg.Service, msg.Container,
					0, "", // default period and no filter
				)
			} else {
				m.activeView = logs_view
				m.logsView.ApplySize(m.window.height-1, m.window.width)
				cmd = m.logsView.Update(msg)
			}
		} else {
			// Need to resolve containers first
			cmd = m.resolveContainers(msg.Cluster, msg.Service, resolvePurposeLogs)
		}
	case messages.ContainersResolved:
		if len(msg.Containers) == 1 {
			// Single container, skip picker
			if m.config.LogsViewer != "" {
				cmd = logsview.OpenInExternalViewer(
					m.config.LogsViewer,
					m.config.ECSClient,
					m.config.CloudWatchLogsClient,
					msg.Cluster, msg.Service, msg.Containers[0],
					0, "", // default period and no filter
				)
			} else {
				m.activeView = logs_view
				m.logsView.ApplySize(m.window.height-1, m.window.width)
				cmd = m.logsView.Update(messages.OpenLogs{
					Cluster:   msg.Cluster,
					Service:   msg.Service,
					Container: msg.Containers[0],
				})
			}
		} else if len(msg.Containers) > 1 {
			// Multiple containers, show picker
			m.dialogs.containerPicker.SetContainers(msg.Cluster, msg.Service, msg.Containers, dialogs.PickerPurposeLogs)
			m.dialogs.open = true
			m.dialogs.active = container_picker_dialog
		} else {
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("no log groups found for service %s", msg.Service),
				}
			}
		}
	case messages.CloseContainerPicker:
		m.dialogs.open = false
	case logsview.ExternalViewerFinishedMsg:
		if msg.Err != nil {
			// Exit status 130 = SIGINT (Ctrl+C) — normal exit from viewer
			// Also suppress "signal: interrupt" which Go reports for signal kills
			var exitErr *exec.ExitError
			if errors.As(msg.Err, &exitErr) && (exitErr.ExitCode() == 130 || exitErr.ExitCode() == -1) {
				break
			}
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("logs viewer: %w", msg.Err),
				}
			}
		}
	case messages.OpenLogsWithEditor:
		if msg.Container != "" {
			// Container known, open editor dialog
			m.dialogs.open = true
			m.dialogs.active = logs_command_dialog
			cmd = m.dialogs.logsCommand.Open(msg.Cluster, msg.Service, msg.Container, m.config.LogsViewer)
		} else {
			// Resolve containers first, then open editor
			cmd = m.resolveContainers(msg.Cluster, msg.Service, resolvePurposeEditor)
		}
	case messages.ContainersResolvedForEditor:
		if len(msg.Containers) == 1 {
			m.dialogs.open = true
			m.dialogs.active = logs_command_dialog
			cmd = m.dialogs.logsCommand.Open(msg.Cluster, msg.Service, msg.Containers[0], m.config.LogsViewer)
		} else if len(msg.Containers) > 1 {
			// Show container picker, then open editor after selection
			// For now, use first container — TODO: chain picker → editor
			m.dialogs.open = true
			m.dialogs.active = logs_command_dialog
			cmd = m.dialogs.logsCommand.Open(msg.Cluster, msg.Service, msg.Containers[0], m.config.LogsViewer)
		} else {
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("no log groups found for service %s", msg.Service),
				}
			}
		}
	case messages.CloseLogsCommandEditor:
		m.dialogs.open = false
	case messages.RunLogsWithCommand:
		cmd = logsview.OpenInExternalViewer(
			msg.Command,
			m.config.ECSClient,
			m.config.CloudWatchLogsClient,
			msg.Cluster, msg.Service, msg.Container,
			msg.Period, msg.FilterPattern,
		)
	case messages.OpenEnvVars:
		if msg.Container != "" {
			m.activeView = envvars_view
			m.envVarsView.ApplySize(m.window.height-1, m.window.width)
			cmd = m.envVarsView.Open(msg.Cluster, msg.Service, msg.Container)
		} else {
			cmd = m.resolveContainers(msg.Cluster, msg.Service, resolvePurposeEnv)
		}
	case messages.ContainersResolvedForEnv:
		if len(msg.Containers) == 1 {
			m.activeView = envvars_view
			m.envVarsView.ApplySize(m.window.height-1, m.window.width)
			cmd = m.envVarsView.Open(msg.Cluster, msg.Service, msg.Containers[0])
		} else if len(msg.Containers) > 1 {
			// Multiple containers — show picker
			m.dialogs.containerPicker.SetContainers(msg.Cluster, msg.Service, msg.Containers, dialogs.PickerPurposeEnvVars)
			m.dialogs.open = true
			m.dialogs.active = container_picker_dialog
		} else {
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("no containers found for service %s", msg.Service),
				}
			}
		}
	case messages.CloseEnvVars:
		m.activeView = tasks_view
	case messages.OpenSSM:
		cmd = startSSMSession(msg.InstanceID, msg.Region, msg.Profile)
	case messages.HostShell:
		cmd = startSSMSession(msg.EC2InstanceID, msg.Region, msg.Profile)
	case messages.ContainersResolvedForHostShell:
		if len(msg.Instances) == 1 {
			cmd = func() tea.Msg {
				return messages.HostShell{
					EC2InstanceID: msg.Instances[0],
					Region:        m.config.Region,
					Profile:       profileString(m.config.Profile),
				}
			}
		} else if len(msg.Instances) > 1 {
			m.dialogs.containerPicker.SetContainers(msg.Cluster, "", msg.Instances, dialogs.PickerPurposeHostShell)
			m.dialogs.containerPicker.SetHostShellContext(m.config.Region, profileString(m.config.Profile))
			m.dialogs.open = true
			m.dialogs.active = container_picker_dialog
		} else {
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("no EC2 hosts in scope"),
				}
			}
		}
	case messages.SSMFinishedMsg:
		if msg.Err != nil {
			var exitErr *exec.ExitError
			if errors.As(msg.Err, &exitErr) && (exitErr.ExitCode() == 130 || exitErr.ExitCode() == -1) {
				break
			}
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("SSM session: %w", msg.Err),
				}
			}
		}
	case messages.ExecIntoContainer:
		cmd = m.startExecSession(msg)
	case messages.ExecFinishedMsg:
		if msg.Err != nil {
			var exitErr *exec.ExitError
			if errors.As(msg.Err, &exitErr) && (exitErr.ExitCode() == 130 || exitErr.ExitCode() == -1) {
				break
			}
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("exec session: %w", msg.Err),
				}
			}
		}
	case messages.ExecSessionReady:
		cmd = startExecSession(msg.SessionJSON, msg.Region, msg.Profile, msg.Target)
	case messages.ContainersResolvedForExec:
		if len(msg.Containers) == 1 {
			cmd = func() tea.Msg {
				return messages.ExecIntoContainer{
					Cluster:   msg.Cluster,
					Service:   msg.Service,
					Task:      msg.Task,
					Container: msg.Containers[0],
					Region:    m.config.Region,
					Profile:   profileString(m.config.Profile),
				}
			}
		} else if len(msg.Containers) > 1 {
			m.dialogs.containerPicker.SetContainers(msg.Cluster, msg.Service, msg.Containers, dialogs.PickerPurposeExec)
			m.dialogs.containerPicker.SetExecContext(msg.Task, m.config.Region, profileString(m.config.Profile))
			m.dialogs.open = true
			m.dialogs.active = container_picker_dialog
		} else {
			cmd = func() tea.Msg {
				return messages.ToggleNotificationDialog{
					Error: fmt.Errorf("no containers found for task"),
				}
			}
		}
	case messages.OpenConsole:
		cmd = openURL(msg.URL)
	}

	var fwdCmd tea.Cmd
	m, fwdCmd = m.forward(msg)
	return m, tea.Batch(cmd, fwdCmd)
}

// forward takes a message and decides to broadcast or to forward only to active
// children
func (m Model) forward(msg tea.Msg) (Model, tea.Cmd) {
	if _, isKeyPress := msg.(tea.KeyPressMsg); isKeyPress {
		return m.routeToActiveOnly(msg)
	}
	return m.broadcast(msg)
}

// broadcast takes a message and forwards it to all children
func (m Model) broadcast(msg tea.Msg) (Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	// views
	cmds = append(cmds, m.clusterSelection.Update(msg))
	cmds = append(cmds, m.serviceSelection.Update(msg))
	cmds = append(cmds, m.taskSelection.Update(msg))
	cmds = append(cmds, m.logsView.Update(msg))
	cmds = append(cmds, m.envVarsView.Update(msg))

	// dialogs
	cmds = append(cmds, m.dialogs.help.Update(msg))
	cmds = append(cmds, m.dialogs.region.Update(msg))
	cmds = append(cmds, m.dialogs.mfa.Update(msg))

	return m, tea.Batch(cmds...)
}

// routeToActiveOnly takes a message and only routes it to a single child, the
// active child with highest precedence (dialogs take precedence over views)
func (m Model) routeToActiveOnly(msg tea.Msg) (Model, tea.Cmd) {
	// exclusively forward keypresses to dialogs if open
	if m.dialogs.open {
		switch m.dialogs.active {
		case help_dialog:
			return m, m.dialogs.help.Update(msg)
		case regions_dialog:
			return m, m.dialogs.region.Update(msg)
		case mfa_dialog:
			return m, m.dialogs.mfa.Update(msg)
		case container_picker_dialog:
			return m, m.dialogs.containerPicker.Update(msg)
		case logs_command_dialog:
			return m, m.dialogs.logsCommand.Update(msg)
		}
	}

	switch m.activeView {
	case clusters_view:
		return m, m.clusterSelection.Update(msg)
	case services_view:
		return m, m.serviceSelection.Update(msg)
	case tasks_view:
		return m, m.taskSelection.Update(msg)
	case logs_view:
		return m, m.logsView.Update(msg)
	case envvars_view:
		return m, m.envVarsView.Update(msg)
	default:
		log.Fatalf("could not identify active view '%d'", int(m.activeView))
	}

	return m, nil
}

func (m Model) switchRegion(oldr, newr string) (Model, tea.Cmd) {
	m.config.Region = newr
	return m, m.Init()
}

// containerResolvePurpose determines what message to emit after resolving containers.
type containerResolvePurpose int

const (
	resolvePurposeLogs containerResolvePurpose = iota
	resolvePurposeEditor
	resolvePurposeEnv
)

func (m Model) resolveContainers(cluster, service string, purpose containerResolvePurpose) tea.Cmd {
	config := m.config
	return func() tea.Msg {
		ecsClient := config.ECSClient
		if ecsClient == nil {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("ECS client not available"),
			}
		}

		var containers []string
		var err error

		if purpose == resolvePurposeEnv {
			// For env vars, resolve all containers from task definition
			containers, err = envvarsadapter.ResolveContainers(ecsClient, context.Background(), cluster, service)
		} else {
			// For logs, resolve only containers with CloudWatch log groups
			groups, resolveErr := cwladapter.ResolveLogGroups(ecsClient, context.Background(), cluster, service)
			if resolveErr != nil {
				err = resolveErr
			} else {
				containers = make([]string, len(groups))
				for i, g := range groups {
					containers[i] = g.Container
				}
			}
		}

		if err != nil {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("resolving containers: %w", err),
			}
		}

		switch purpose {
		case resolvePurposeEditor:
			return messages.ContainersResolvedForEditor{
				Cluster:    cluster,
				Service:    service,
				Containers: containers,
			}
		case resolvePurposeEnv:
			return messages.ContainersResolvedForEnv{
				Cluster:    cluster,
				Service:    service,
				Containers: containers,
			}
		default: // resolvePurposeLogs
			return messages.ContainersResolved{
				Cluster:    cluster,
				Service:    service,
				Containers: containers,
			}
		}
	}
}

func (m Model) applySize(height, width int) tea.Model {
	m.Help.SetWidth(width)
	m.window.height = height
	m.window.width = width
	return m
}

func (m Model) handleSwitchView(msg messages.SwitchView) (Model, tea.Cmd) {
	switch msg.NewView {
	case messages.Cluster_selection:
		m.activeView = clusters_view
	case messages.Service_selection:
		m.activeView = services_view
	case messages.Task_selection:
		m.activeView = tasks_view
	case messages.Logs_view:
		m.activeView = logs_view
	}
	return m, m.dialogs.help.Update(msg)
}

func (m Model) handleErrorTick(msg messages.NotificationTick) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, d := range m.dialogs.notification {
		cmds = append(cmds, d.Update(msg))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) HandleExpiredError(msg messages.NotificationExpired) (Model, tea.Cmd) {
	if idx := u.FindBy(m.dialogs.notification, func(d *dialogs.NotificationDialog) bool {
		return d != nil && d.ID() == msg.ID
	}); idx >= 0 {
		m.dialogs.notification = slices.Delete(m.dialogs.notification, idx, idx+1)
	}
	return m, nil
}

func (m Model) ToggleNotificationDialog(msg messages.ToggleNotificationDialog) (Model, tea.Cmd) {
	options := []dialogs.Option{}
	if msg.Duration != 0 {
		options = append(options, dialogs.WithDuration(msg.Duration))
	}
	d := dialogs.NewNotificationDialog(msg.Msg, msg.Error, options...)
	m.dialogs.notification = append(m.dialogs.notification, d)
	return m, d.Tick() // initialise ticking
}

func (m Model) ToggleHelpDialog() (Model, tea.Cmd) {
	if m.dialogs.open && m.dialogs.active != help_dialog {
		return m, nil
	}
	m.dialogs.open = !m.dialogs.open
	if m.dialogs.open {
		m.dialogs.active = help_dialog
	}
	return m, nil
}

func (m Model) ToggleRegionsDialog() (Model, tea.Cmd) {
	if m.dialogs.open && m.dialogs.active != regions_dialog {
		return m, nil
	}
	m.dialogs.open = !m.dialogs.open
	if m.dialogs.open {
		m.dialogs.active = regions_dialog
	}
	return m, nil
}

func (m Model) OpenMFADialog() (Model, tea.Cmd) {
	m.dialogs.open = true
	m.dialogs.active = mfa_dialog
	return m, m.dialogs.mfa.Update(messages.MFAFocus{}) // init focus
}

// TODO: now assuming no dialog can be open prior to MFA call; fallback to
// previous dialog if appliccable!
func (m Model) CloseMFADialog() (Model, tea.Cmd) {
	m.dialogs.open = false
	return m, nil
}

type dialog interface {
	View() string
}

func (m Model) View() tea.View {
	var page string
	var help string
	switch m.activeView {
	case clusters_view:
		page = m.clusterSelection.View()
		help = m.Help.ShortHelpView(m.clusterSelection.ShortHelp())
	case services_view:
		page = m.serviceSelection.View()
		help = m.Help.ShortHelpView(m.serviceSelection.ShortHelp())
	case tasks_view:
		page = m.taskSelection.View()
		help = m.Help.ShortHelpView(m.taskSelection.ShortHelp())
	case logs_view:
		page = m.logsView.View()
		help = m.Help.ShortHelpView(m.logsView.ShortHelp())
	case envvars_view:
		page = m.envVarsView.View()
		help = m.Help.ShortHelpView(m.envVarsView.ShortHelp())
	}

	// assemble gutter
	region := regionBlock.Render(m.config.Region)
	gutter := lipgloss.JoinHorizontal(lipgloss.Left, region, " ", help)

	page = lipgloss.JoinVertical(lipgloss.Top, page, gutter)

	// dialog compositing
	mainLayer := lipgloss.NewLayer(page)
	c := lipgloss.NewCompositor(mainLayer)
	c.AddLayers(mainLayer)
	if m.dialogs.open {
		var dialog dialog
		switch m.dialogs.active {
		case help_dialog:
			dialog = m.dialogs.help
		case regions_dialog:
			dialog = m.dialogs.region
		case mfa_dialog:
			dialog = m.dialogs.mfa
		case container_picker_dialog:
			dialog = m.dialogs.containerPicker
		case logs_command_dialog:
			dialog = m.dialogs.logsCommand
		}
		renderedDialog := dialog.View()
		dialogLayer := lipgloss.NewLayer(renderedDialog).
			X(m.window.width/2 - lipgloss.Width(renderedDialog)/2).
			Y(m.window.height/2 - lipgloss.Height(renderedDialog)/2)
		c.AddLayers(dialogLayer)
	}

	// error messages
	var errors []string
	for _, d := range m.dialogs.notification {
		errors = append(errors, d.View())
	}
	if len(errors) > 0 {
		errorContent := lipgloss.JoinVertical(lipgloss.Left, errors...)
		errorLayer := lipgloss.NewLayer(errorContent).X(1).Y(1)
		c.AddLayers(errorLayer)
	}

	v := tea.NewView(c.Render())
	v.AltScreen = true // fullscreen
	return v
}

func (m Model) SignalOpenHelpDialog() tea.Cmd {
	return func() tea.Msg {
		return messages.ToggleHelp{}
	}
}

// startExecSession calls the ECS ExecuteCommand connector and, on success,
// suspends the TUI to launch session-manager-plugin. Errors are surfaced as a
// notification.
func (m Model) startExecSession(msg messages.ExecIntoContainer) tea.Cmd {
	client := m.config.ECSClient
	region := msg.Region
	if region == "" {
		region = m.config.Region
	}
	profile := msg.Profile
	return func() tea.Msg {
		if client == nil {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("ECS client not available"),
			}
		}
		out, err := ecsconnector.ExecuteCommand(client, context.Background(), msg.Cluster, msg.Task, msg.Container, "/bin/sh")
		if err != nil {
			return messages.ToggleNotificationDialog{
				Error: fmt.Errorf("exec: %w", err),
			}
		}
		return messages.ExecSessionReady{
			SessionJSON: string(out.Session),
			Region:      region,
			Profile:     profile,
			Target:      msg.Task,
		}
	}
}

// profileString safely dereferences a possibly-nil profile pointer.
func profileString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
