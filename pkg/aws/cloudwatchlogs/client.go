package cloudwatchlogs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// Client holds the AWS SDK client for CloudWatch Logs operations.
type Client struct {
	Logs *cloudwatchlogs.Client
}

// NewClient creates a new CloudWatch Logs client from an AWS config.
func NewClient(cfg *aws.Config) *Client {
	return &Client{
		Logs: cloudwatchlogs.NewFromConfig(*cfg),
	}
}
