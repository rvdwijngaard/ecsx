package messages

import (
	"time"

	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	cwtypes "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatch/types"
	cwltypes "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatchlogs/types"
	apitypes "github.com/ron/ecsx/pkg/ui/internal/adapters/dynamodb/types"
	apitypes2 "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs/types"
)

type View int
type ItemsQueryMode int

const (
	Table_selection View = iota
	Item_selection
	Service_selection
	Task_selection
	Logs_view
)
const (
	ScanMode ItemsQueryMode = iota
	QueryMode
)

type QueryOperator string

const (
	Noop         QueryOperator = ""
	Equals       QueryOperator = "equals"
	GreaterEqual QueryOperator = "greater than or equal"
	Greater      QueryOperator = "greater than"
	LessEqual    QueryOperator = "less than or equal"
	Less         QueryOperator = "less than"
	Between      QueryOperator = "between"
	BeginsWith   QueryOperator = "begins with"
)

type TableIndex struct {
	HashKey     string
	HashKeyType string

	RangeKey     *string
	RangeKeyType string
}

type GlobalSecondaryIndex struct {
	Name string

	HashKey     string
	HashKeyType string

	RangeKey     *string
	RangeKeyType string
}

type LocalSecondaryIndex struct {
	Name string

	HashKey     string
	HashKeyType string

	RangeKey     string
	RangeKeyType string
}

type SwitchView struct {
	OldView View
	NewView View
}

type SelectTable struct {
	TableName    string
	TableDetails apitypes.DescribeTableResponse
}

type ZoomToggleItemSelectionPane struct{}
type ZoomToggleItemDetailsPane struct{}
type ZoomToggleTableSelectionPane struct{}
type ZoomToggleTableDetailsPane struct{}
type ZoomToggleServiceSelectionPane struct{}
type ZoomToggleServiceDetailsPane struct{}
type ZoomToggleTaskSelectionPane struct{}
type ZoomToggleTaskDetailsPane struct{}

type PreviewItem struct {
	StyledItem string
	RawItem    string
}

type TableDetails struct {
	Details *apitypes.DescribeTableResponse
}

type ToggleJSONYAML struct{}

type Page struct {
	Items            apitypes.Items
	LastEvaluatedKey map[string]dynamodbtypes.AttributeValue
}

type PageReady struct {
	Table    apitypes.DescribeTableResponse
	Index    *string
	Response *Page
	Err      error
}

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
	Cluster  string
	Service  string
	Tasks    []apitypes2.TaskItem
	Err      error
}

type TaskDetails struct {
	Details *apitypes2.TaskItem
}

type TablePageReady struct {
	Tables        []string
	PaginationKey *string
	Err           error
	Region        string
}

type ToggleHelp struct{}
type ToggleRegions struct{}

type ToggleColumnVisibility struct{}
type ToggleColumnSorting struct{}
type ToggleScanParameters struct{}
type ToggleQueryParameters struct{}
type ToggleNotificationDialog struct {
	Msg      string
	Error    error
	Duration time.Duration
}

type ToggleColumnCopy struct{}

type CloseMFADialog struct{}
type MFAFocus struct{}

type NotificationTick struct{ ID string }
type NotificationExpired struct{ ID string }

type InitColumnVisibility struct {
	TableARN   string
	AllColumns []string // matching by index
	Visible    []bool   // matching by index
}

type InitColumnSorting struct {
	TableARN   string
	AllColumns []string // matching by index
	SortingOn  string
	Ascending  bool // if false, descending
}

type InitScanParameters struct {
	TableARN     string
	TableIndex   TableIndex
	GSI          []GlobalSecondaryIndex
	LSI          []LocalSecondaryIndex
	CurrentIndex *string
}

type InitQueryParameters struct {
	TableARN             string
	TableIndex           TableIndex
	GSI                  []GlobalSecondaryIndex
	LSI                  []LocalSecondaryIndex
	CurrentIndex         *string
	HashKeyValue         string
	RangeKeyValue1       *string
	RangeKeyValue2       *string // used for BETWEEN operator
	RangeKeyOperator     QueryOperator
	RangeOrderDescending bool // ascending by default
}

type InitColumnCopy struct {
	TableARN   string
	AllColumns []string // matching by index
	ColValues  []string // matching by index
}

type ColumnVisibilityUpdate struct {
	TableARN   string
	AllColumns []string // matching by index
	Visible    []bool   // matching by index
}

type ColumnSortingUpdate struct {
	TableARN   string
	AllColumns []string // matching by index
	SortingOn  string
	Ascending  bool // if false, descending
}

type ScanIndexChanged struct {
	TableARN  string
	IndexName string // empty == table index
}

type QueryParametersChanged struct {
	TableARN             string
	IndexName            string // empty == table index
	HashKeyValue         string
	RangeKeyValue1       *string
	RangeKeyValue2       *string // used for BETWEEN operator
	RangeKeyOperator     QueryOperator
	RangeOrderDescending bool // ascending by default
}

type ColumnSortingReset struct {
	TableARN string
}

type SwitchRegion struct {
	OldRegion string
	NewRegion string
}

type SwitchQueryMode struct {
	OldMode ItemsQueryMode
	NewMode ItemsQueryMode
}

type CopyItem struct{}

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

// --- Force new deployment messages ---

type ForceNewDeployment struct {
	Cluster string
	Service string
}

type ForceNewDeploymentResult struct {
	Cluster string
	Service string
	Err     error
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
