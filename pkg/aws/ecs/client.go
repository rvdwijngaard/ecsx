package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Client holds the AWS SDK client for ECS operations.
type Client struct {
	ECS *ecs.Client

	region  string
	profile string
}

// NewClient creates a new ECS client from an AWS config.
func NewClient(cfg *aws.Config, profile string) *Client {
	return &Client{
		ECS:     ecs.NewFromConfig(*cfg),
		region:  cfg.Region,
		profile: profile,
	}
}

func (c *Client) Region() string  { return c.region }
func (c *Client) Profile() string { return c.profile }
