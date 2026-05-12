package types

import (
	"time"

	ecstypes "github.com/ron/ecsx/pkg/aws/ecs/types"
)

// ClusterItem represents a cluster formatted for UI display.
type ClusterItem struct {
	Name               string
	ARN                string
	Status             string
	ContainerInstances int32
	ActiveServices     int32
	RunningTasks       int32
	PendingTasks       int32
}

// ClusterFromConnector converts a connector cluster type to an adapter cluster type.
func ClusterFromConnector(c ecstypes.Cluster) ClusterItem {
	return ClusterItem{
		Name:               c.Name,
		ARN:                c.ARN,
		Status:             c.Status,
		ContainerInstances: c.ContainerInstances,
		ActiveServices:     c.ActiveServices,
		RunningTasks:       c.RunningTasks,
		PendingTasks:       c.PendingTasks,
	}
}

// ServiceItem represents a service formatted for UI display.
type ServiceItem struct {
	Name           string
	ARN            string
	Status         string
	TaskDefinition string
	DesiredCount   int32
	RunningCount   int32
	PendingCount   int32
	LaunchType     string
	CreatedAt      *time.Time
}

// TaskItem represents a task formatted for UI display.
type TaskItem struct {
	ID                  string
	ARN                 string
	Status              string
	DesiredStatus       string
	HealthStatus        string
	LaunchType          string
	TaskDefinition      string
	Group               string
	CPU                 string
	Memory              string
	StartedAt           *time.Time
	CreatedAt           *time.Time
	StoppedAt           *time.Time
	StoppedReason       string
	ContainerInstanceID string
	EC2InstanceID       string
	PrivateIP           string
	PublicIP            string
	Containers          []ContainerItem
	// Context fields for console URLs
	ClusterName string
	Region      string
}

// ContainerItem represents a container formatted for UI display.
type ContainerItem struct {
	Name         string
	Status       string
	HealthStatus string
	Image        string
	ExitCode     *int32
}

// TaskFromConnector converts a connector task type to an adapter task type.
func TaskFromConnector(t ecstypes.Task) TaskItem {
	containers := make([]ContainerItem, len(t.Containers))
	for i, c := range t.Containers {
		containers[i] = ContainerItem{
			Name:         c.Name,
			Status:       c.Status,
			HealthStatus: c.HealthStatus,
			Image:        c.Image,
			ExitCode:     c.ExitCode,
		}
	}
	return TaskItem{
		ID:                  t.ID,
		ARN:                 t.ARN,
		Status:              t.Status,
		DesiredStatus:       t.DesiredStatus,
		HealthStatus:        t.HealthStatus,
		LaunchType:          t.LaunchType,
		TaskDefinition:      t.TaskDefinition,
		Group:               t.Group,
		CPU:                 t.CPU,
		Memory:              t.Memory,
		StartedAt:           t.StartedAt,
		CreatedAt:           t.CreatedAt,
		StoppedAt:           t.StoppedAt,
		StoppedReason:       t.StoppedReason,
		ContainerInstanceID: t.ContainerInstanceID,
		EC2InstanceID:       t.EC2InstanceID,
		PrivateIP:           t.PrivateIP,
		PublicIP:            t.PublicIP,
		Containers:          containers,
	}
}
