package appconfig

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Config holds application-level configuration for ecsx.
type Config struct {
	Profile *string
	Region  string
	Cluster string
	Verbose bool

	AvailableRegions []string
	StarredRegions   []string
	LogsViewer       string

	ECSClient            *ecs.Client
	CloudWatchClient     *cloudwatch.Client
	CloudWatchLogsClient *cloudwatchlogs.Client
	// MFA credentials support
	MFACredentialCB func() (string, error)
	MFACredentialC  chan<- CredentialsResponse
}

type CredentialsRequest struct{}

type CredentialsResponse struct {
	Token string
	Error error
}
