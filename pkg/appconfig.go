package appconfig

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Config holds application-level configuration for ecsx.
type Config struct {
	Profile *string
	Region  string
	Cluster string
	Verbose bool

	MaxTables        int
	URL              *string
	AvailableRegions []string
	StarredRegions   []string
	LogsViewer       string

	Client               *dynamodb.Client
	ECSClient            *ecs.Client
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
