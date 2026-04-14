package ui

import (
	"fmt"
	"time"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

type viewLevel int

const (
	viewClusters viewLevel = iota
	viewServices
	viewTasks
	viewLogs
	viewEC2Instances
)

// Messages
type clustersLoadedMsg struct{ clusters []ecsaws.Cluster }
type serviceNamesLoadedMsg struct{ names []string }
type serviceDetailLoadedMsg struct {
	name    string
	service *ecsaws.Service
}
type servicesLoadedMsg struct{ services []ecsaws.Service }
type tasksLoadedMsg struct{ tasks []ecsaws.Task }
type envVarsLoadedMsg struct{ containers []ecsaws.ContainerDefinition }
type serviceScaledMsg struct{}
type serviceRedeployedMsg struct{}
type taskStoppedMsg struct{}
type metricsLoadedMsg struct {
	service string
	metrics *ecsaws.ServiceMetrics
}
type logEventMsg struct{ event ecsaws.LogEvent }
type logErrorMsg struct{ err error }
type logGroupFoundMsg struct {
	logGroup     string
	streamPrefix string
}
type errMsg struct{ err error }

// EC2 instance picker
type ec2InstancesLoadedMsg struct{ instances []ecsaws.ContainerInstance }
type ssmTargetMsg struct{ instanceID string }

type ec2Item struct{ instance ecsaws.ContainerInstance }

func (i ec2Item) Title() string { return i.instance.EC2InstanceID }
func (i ec2Item) Description() string {
	return fmt.Sprintf("%s running=%d pending=%d", i.instance.Status, i.instance.RunningTasks, i.instance.PendingTasks)
}
func (i ec2Item) FilterValue() string { return i.instance.EC2InstanceID }

// List items
type clusterItem struct{ cluster ecsaws.Cluster }

func (i clusterItem) Title() string { return i.cluster.Name }
func (i clusterItem) Description() string {
	return fmt.Sprintf("services=%d running=%d pending=%d",
		i.cluster.ActiveServices, i.cluster.RunningTasks, i.cluster.PendingTasks)
}
func (i clusterItem) FilterValue() string { return i.cluster.Name }

type serviceItem struct {
	name    string
	service *ecsaws.Service
}

func (i serviceItem) Title() string { return i.name }
func (i serviceItem) Description() string {
	if i.service == nil {
		return "loading..."
	}
	return fmt.Sprintf("%s desired=%d running=%d", i.service.Status, i.service.DesiredCount, i.service.RunningCount)
}
func (i serviceItem) FilterValue() string { return i.name }

type taskItem struct{ task ecsaws.Task }

func (i taskItem) Title() string { return i.task.ID }
func (i taskItem) Description() string {
	desc := fmt.Sprintf("%s %s", i.task.LaunchType, i.task.Status)
	if i.task.StartedAt != nil {
		desc += fmt.Sprintf(" up %s", formatDuration(time.Since(*i.task.StartedAt)))
	}
	if i.task.HealthStatus != "" && i.task.HealthStatus != "UNKNOWN" {
		desc += fmt.Sprintf(" [%s]", i.task.HealthStatus)
	}
	return desc
}
func (i taskItem) FilterValue() string { return i.task.ID }

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}