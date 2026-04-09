package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

func (m Model) loadClusters() tea.Cmd {
	return func() tea.Msg {
		clusters, err := m.client.ListClusters(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return clustersLoadedMsg{clusters}
	}
}

func (m Model) loadServices(cluster string) tea.Cmd {
	return func() tea.Msg {
		services, err := m.client.ListServices(context.Background(), cluster)
		if err != nil {
			return errMsg{err}
		}
		return servicesLoadedMsg{services}
	}
}

func (m Model) loadServiceNames(cluster string) tea.Cmd {
	return func() tea.Msg {
		names, err := m.client.ListServiceNames(context.Background(), cluster)
		if err != nil {
			return errMsg{err}
		}
		return serviceNamesLoadedMsg{names}
	}
}

func (m Model) loadServiceDetail(cluster, service string) tea.Cmd {
	return func() tea.Msg {
		svc, err := m.client.DescribeService(context.Background(), cluster, service)
		if err != nil {
			return errMsg{err}
		}
		return serviceDetailLoadedMsg{name: service, service: svc}
	}
}

func (m Model) loadTasks(cluster, service string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := m.client.ListTasks(context.Background(), cluster, service)
		if err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{tasks}
	}
}

func (m Model) loadEnvVars(taskDef string) tea.Cmd {
	return func() tea.Msg {
		containers, err := m.client.DescribeTaskDefinition(context.Background(), taskDef)
		if err != nil {
			return errMsg{err}
		}
		return envVarsLoadedMsg{containers}
	}
}

func (m Model) loadMetrics(cluster, service string) tea.Cmd {
	return func() tea.Msg {
		metrics, err := m.client.GetServiceMetrics(context.Background(), cluster, service)
		if err != nil {
			return metricsLoadedMsg{service: service, metrics: &ecsaws.ServiceMetrics{}}
		}
		return metricsLoadedMsg{service: service, metrics: metrics}
	}
}

func (m Model) loadContainerInstances(cluster string) tea.Cmd {
	return func() tea.Msg {
		instances, err := m.client.ListContainerInstances(context.Background(), cluster)
		if err != nil {
			return errMsg{err}
		}
		return ec2InstancesLoadedMsg{instances}
	}
}
