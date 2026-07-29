package messages

import (
	"time"

	cwtypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatch/types"
	cwltypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
	apitypes2 "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs/types"
)

type View int

const (
	Cluster_selection View = iota
	Service_selection
	Task_selection
	Logs_view
)

type SwitchView struct {
	OldView View
	NewView View
}

type ZoomToggleServiceSelectionPane struct{}
type ZoomToggleServiceDetailsPane struct{}
type ZoomToggleTaskSelectionPane struct{}
type ZoomToggleTaskDetailsPane struct{}
type ZoomToggleClusterSelectionPane struct{}
type ZoomToggleClusterDetailsPane struct{}

type ClusterPageReady struct {
	Clusters []apitypes2.ClusterItem
	Err      error
}

type ClusterDetails struct {
	Details *apitypes2.ClusterItem
}

type SelectCluster struct {
	ClusterName string
	Details     apitypes2.ClusterItem
}

type ServicePageReady struct {
	Cluster  string
	Services []apitypes2.ServiceItem
	Err      error
}

type ServiceDetails struct {
	Details *apitypes2.ServiceItem
}

type SelectService struct {
	ClusterName string
	ServiceName string
	Details     apitypes2.ServiceItem
}

type ServiceMetricsReady struct {
	Cluster string
	Service string
	Metrics *cwtypes.ServiceMetrics
	Err     error
}

type TaskPageReady struct {
	Cluster string
	Service string
	Tasks   []apitypes2.TaskItem
	Err     error
}

type TaskDetails struct {
	Details *apitypes2.TaskItem
}

type ToggleHelp struct{}
type ToggleRegions struct{}

type ToggleNotificationDialog struct {
	Msg      string
	Error    error
	Duration time.Duration
}

type CloseMFADialog struct{}
type MFAFocus struct{}

type NotificationTick struct{ ID string }
type NotificationExpired struct{ ID string }

type SwitchRegion struct {
	OldRegion string
	NewRegion string
}

// --- Logs view messages ---

type OpenLogs struct {
	Cluster   string
	Service   string
	Container string // empty = pick first available
}

type CloseLogs struct{}

type LogBatch struct {
	Lines []cwltypes.FormattedLogLine
}

type LogTailError struct {
	Err error
}

type CloseContainerPicker struct{}

type ContainersResolved struct {
	Cluster    string
	Service    string
	Containers []string
}

type CloseLogsCommandEditor struct{}

type RunLogsWithCommand struct {
	Cluster       string
	Service       string
	Container     string
	Command       string
	Period        time.Duration
	FilterPattern string
}

type OpenLogsWithEditor struct {
	Cluster   string
	Service   string
	Container string
}

type ContainersResolvedForEditor struct {
	Cluster    string
	Service    string
	Containers []string
}

// --- Env vars messages ---

type OpenEnvVars struct {
	Cluster   string
	Service   string
	Container string // empty = pick first or show picker
}

type EnvVarsReady struct {
	Cluster   string
	Service   string
	Container string
	Err       error
}

type CloseEnvVars struct{}

type ContainersResolvedForEnv struct {
	Cluster    string
	Service    string
	Containers []string
}

// --- SSM session messages ---

type OpenSSM struct {
	InstanceID string
	Region     string
	Profile    string
}

type SSMFinishedMsg struct {
	Err error
}

// HostShell requests an interactive SSM session into an EC2 container
// instance, identified by its EC2 instance ID.
type HostShell struct {
	EC2InstanceID string
	Region        string
	Profile       string
}

// ContainersResolvedForHostShell is emitted after resolving the EC2 instances
// in scope for a host-shell session so the user can pick one.
type ContainersResolvedForHostShell struct {
	Cluster   string
	Instances []string
}

// OpenConsole asks the model to open the given URL in the user's default
// browser.
type OpenConsole struct {
	URL string
}

// --- ECS exec session messages ---

// ExecIntoContainer requests an interactive shell session into a container.
// When Container is empty, the caller must resolve containers first.
type ExecIntoContainer struct {
	Cluster   string
	Service   string
	Task      string // task ARN
	Container string // container name
	Region    string
	Profile   string
}

type ExecFinishedMsg struct {
	Err error
}

// ExecSessionReady carries the data needed to launch session-manager-plugin
// after the ExecuteCommand API call has succeeded.
type ExecSessionReady struct {
	SessionJSON string
	Region      string
	Profile     string
	Target      string // ECS task ARN — required by session-manager-plugin
}

// ContainersResolvedForExec is emitted after resolving the containers of a
// task so the user can pick one before exec.
type ContainersResolvedForExec struct {
	Cluster    string
	Service    string
	Task       string
	Containers []string
}
