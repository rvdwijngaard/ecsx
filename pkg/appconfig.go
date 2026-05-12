package appconfig

import "github.com/aws/aws-sdk-go-v2/service/dynamodb"

// Config holds application-level configuration for ecsx.
type Config struct {
	Profile *string
	Region  string
	Cluster string
	Verbose bool

	Client *dynamodb.Client
	// MFA credentials support
	MFACredentialCB func() (string, error)
	MFACredentialC  chan<- CredentialsResponse
}

type CredentialsRequest struct{}

type CredentialsResponse struct {
	Token string
	Error error
}
