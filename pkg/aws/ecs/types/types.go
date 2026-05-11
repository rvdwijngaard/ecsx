package types

import (
	"encoding/json"
	"time"
)

type Cluster struct {
	Name               string
	ARN                string
	Status             string
	ContainerInstances int32
	ActiveServices     int32
	RunningTasks       int32
	PendingTasks       int32
}

type Service struct {
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

type Container struct {
	Name            string
	Status          string
	ExitCode        *int32
	Reason          string
	HealthStatus    string
	Image           string
	NetworkBindings []NetworkBinding
}

type NetworkBinding struct {
	ContainerPort int32
	HostPort      int32
	Protocol      string
	BindIP        string
}

type Task struct {
	ID                  string
	ARN                 string
	TaskDefinition      string
	Status              string
	LastStatus          string
	DesiredStatus       string
	LaunchType          string
	HealthStatus        string
	Group               string
	CPU                 string
	Memory              string
	StartedAt           *time.Time
	CreatedAt           *time.Time
	StoppedAt           *time.Time
	StoppedReason       string
	ContainerInstanceID string
	EC2InstanceID       string
	Containers          []Container
	// Fargate networking
	PrivateIP string
	PublicIP  string
}

type ContainerInstance struct {
	ARN           string
	EC2InstanceID string
	Status        string
	RunningTasks  int32
	PendingTasks  int32
}

type EnvVar struct {
	Name  string
	Value string
}

type ContainerDefinition struct {
	Name    string
	Image   string
	CPU     int32
	Memory  int32
	EnvVars []EnvVar
}

// ExecuteCommandOutput holds the session info needed to start session-manager-plugin.
type ExecuteCommandOutput struct {
	Session   json.RawMessage
	TaskARN   string
	Container string
	Cluster   string
}
