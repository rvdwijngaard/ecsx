package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	sel := m.list.SelectedItem()
	if sel == nil {
		return m, nil
	}
	switch m.level {
	case viewClusters:
		c := sel.(clusterItem).cluster
		m.clusterName = c.Name
		m.loading = true
		m.err = nil
		return m, tea.Batch(m.spinner.Tick, m.loadServiceNames(c.Name))
	case viewServices:
		s := sel.(serviceItem)
		if s.service == nil {
			return m, nil
		}
		m.serviceName = s.name
		m.loading = true
		m.err = nil
		return m, tea.Batch(m.spinner.Tick, m.loadTasks(m.clusterName, s.name))
	case viewEC2Instances:
		inst := sel.(ec2Item).instance
		return m, startSSMSession(inst.EC2InstanceID, m.client.Region(), m.client.Profile())
	}
	return m, nil
}

func (m Model) handleEsc() (tea.Model, tea.Cmd) {
	if m.err != nil {
		m.err = nil
	}
	if m.showEnvVars {
		m.showEnvVars = false
		m.envVars = nil
		m.updateDetail()
		return m, nil
	}
	switch m.level {
	case viewLogs:
		m.stopLogTail()
		m.zoomed = false
		m.logFilter = ""
		m.logGroup = ""
		m.logStreamPrefix = ""
		if m.serviceName != "" {
			m.level = viewTasks
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.loadTasks(m.clusterName, m.serviceName))
		}
		m.level = viewServices
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadServiceNames(m.clusterName))
	case viewEC2Instances:
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadClusters())
	case viewServices:
		if m.initialCluster != "" {
			return m, tea.Quit
		}
		m.loading = true
		m.clusterName = ""
		return m, tea.Batch(m.spinner.Tick, m.loadClusters())
	case viewTasks:
		m.loading = true
		m.serviceName = ""
		return m, tea.Batch(m.spinner.Tick, m.loadServiceNames(m.clusterName))
	}
	return m, nil
}

func (m Model) handleEnvVars() (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	if m.showEnvVars {
		m.showEnvVars = false
		m.envVars = nil
		m.updateDetail()
		return m, nil
	}
	var taskDef string
	switch m.level {
	case viewServices:
		if sel := m.list.SelectedItem(); sel != nil {
			if s := sel.(serviceItem).service; s != nil {
				taskDef = s.TaskDefinition
			}
		}
	case viewTasks:
		if sel := m.list.SelectedItem(); sel != nil {
			taskDef = sel.(taskItem).task.TaskDefinition
		}
	}
	if taskDef == "" {
		return m, nil
	}
	m.loading = true
	return m, tea.Batch(m.spinner.Tick, m.loadEnvVars(taskDef))
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	if cc, ok := m.client.(*ecsaws.CachedClient); ok {
		cc.Purge()
	}
	m.showEnvVars = false
	m.envVars = nil
	m.loading = true
	m.err = nil
	switch m.level {
	case viewClusters:
		return m, tea.Batch(m.spinner.Tick, m.loadClusters())
	case viewServices:
		return m, tea.Batch(m.spinner.Tick, m.loadServiceNames(m.clusterName))
	case viewTasks:
		return m, tea.Batch(m.spinner.Tick, m.loadTasks(m.clusterName, m.serviceName))
	case viewEC2Instances:
		return m, tea.Batch(m.spinner.Tick, m.loadContainerInstances(m.clusterName))
	case viewLogs:
		m.logFilter = ""
		m.loading = false
		return m, m.startLogTail()
	}
	return m, nil
}

func (m Model) handleScale() (tea.Model, tea.Cmd) {
	if m.loading || m.level != viewServices {
		return m, nil
	}
	sel := m.list.SelectedItem()
	if sel == nil {
		return m, nil
	}
	svc := sel.(serviceItem).service
	if svc == nil {
		return m, nil
	}
	m.scaling = true
	m.scaleSvc = svc.Name
	ti := textinput.New()
	ti.Placeholder = "desired count"
	ti.SetValue(fmt.Sprintf("%d", svc.DesiredCount))
	ti.CharLimit = 5
	ti.SetWidth(20)
	m.scaleInput = ti
	cmd := m.scaleInput.Focus()
	m.detail.SetContent(fmt.Sprintf(
		"%s\n\n  Current desired count: %d\n\n  New desired count: %s\n\n  %s",
		titleStyle.Render("Scale "+svc.Name),
		svc.DesiredCount,
		m.scaleInput.View(),
		helpStyle.Render("enter confirm • esc cancel"),
	))
	return m, cmd
}

func (m Model) confirmScale() (tea.Model, tea.Cmd) {
	val, err := strconv.Atoi(m.scaleInput.Value())
	if err != nil || val < 0 {
		m.err = fmt.Errorf("invalid count: %s", m.scaleInput.Value())
		m.scaling = false
		return m, nil
	}
	m.scaling = false
	m.loading = true
	cluster := m.clusterName
	svc := m.scaleSvc
	count := int32(val)
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		err := m.client.UpdateServiceDesiredCount(context.Background(), cluster, svc, count)
		if err != nil {
			return errMsg{err}
		}
		return serviceScaledMsg{}
	})
}

func (m Model) handleYank() (tea.Model, tea.Cmd) {
	if m.envVars == nil {
		return m, nil
	}
	var b strings.Builder
	for _, cd := range m.envVars {
		for _, ev := range cd.EnvVars {
			fmt.Fprintf(&b, "export %s=%q\n", ev.Name, ev.Value)
		}
	}
	err := copyToClipboard(b.String())
	if err != nil {
		m.err = fmt.Errorf("clipboard: %w", err)
		return m, nil
	}
	return m, nil
}

func (m Model) handleSSM() (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	switch m.level {
	case viewTasks:
		sel := m.list.SelectedItem()
		if sel == nil {
			return m, nil
		}
		t := sel.(taskItem).task
		if t.EC2InstanceID == "" {
			m.err = fmt.Errorf("no EC2 instance (task is %s)", t.LaunchType)
			return m, nil
		}
		return m, startSSMSession(t.EC2InstanceID, m.client.Region(), m.client.Profile())
	case viewServices:
		sel := m.list.SelectedItem()
		if sel == nil {
			return m, nil
		}
		s := sel.(serviceItem)
		if s.service == nil {
			return m, nil
		}
		// Load tasks to find an EC2 instance
		m.loading = true
		cluster := m.clusterName
		svcName := s.name
		client := m.client
		return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
			tasks, err := client.ListTasks(context.Background(), cluster, svcName)
			if err != nil {
				return errMsg{err}
			}
			for _, t := range tasks {
				if t.EC2InstanceID != "" {
					return ssmTargetMsg{instanceID: t.EC2InstanceID}
				}
			}
			return errMsg{fmt.Errorf("no EC2-backed tasks found for service %s", svcName)}
		})
	case viewClusters:
		sel := m.list.SelectedItem()
		if sel == nil {
			return m, nil
		}
		c := sel.(clusterItem).cluster
		m.clusterName = c.Name
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadContainerInstances(c.Name))
	}
	return m, nil
}

func (m Model) handleLogs() (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	var svcName string
	switch m.level {
	case viewServices:
		if sel := m.list.SelectedItem(); sel != nil {
			if s := sel.(serviceItem).service; s != nil {
				svcName = s.Name
			}
		}
	case viewTasks:
		svcName = m.serviceName
	default:
		return m, nil
	}
	if svcName == "" {
		return m, nil
	}
	m.loading = true
	cluster := m.clusterName
	client := m.client
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		logGroup, streamPrefix, err := ecsaws.FindLogGroup(context.Background(), client, cluster, svcName)
		if err != nil {
			return errMsg{err}
		}
		return logGroupFoundMsg{logGroup: logGroup, streamPrefix: streamPrefix}
	})
}
