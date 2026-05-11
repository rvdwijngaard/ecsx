package appconfig

// Config holds application-level configuration for ecsx.
type Config struct {
	Profile *string
	Region  string
	Cluster string
	Verbose bool

	// MFA credentials support
	MFACredentialCB func() (string, error)
	MFACredentialC  chan<- CredentialsResponse
}

type CredentialsRequest struct{}

type CredentialsResponse struct {
	Token string
	Error error
}
