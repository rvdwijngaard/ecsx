package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

// Client holds the AWS SDK client for CloudWatch operations.
type Client struct {
	CW *cw.Client
}

// NewClient creates a new CloudWatch client from an AWS config.
func NewClient(cfg *aws.Config) *Client {
	return &Client{
		CW: cw.NewFromConfig(*cfg),
	}
}
