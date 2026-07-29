package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	cwltypes "github.com/rvdwijngaard/ecsx/pkg/aws/cloudwatchlogs/types"
)

type logGroupClient interface {
	DescribeServices(context.Context, *ecs.DescribeServicesInput, ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	DescribeTaskDefinition(context.Context, *ecs.DescribeTaskDefinitionInput, ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
}

// FindLogGroups resolves all container log groups from a service's task definition.
func FindLogGroups(client logGroupClient, ctx context.Context, cluster, service string) ([]cwltypes.ContainerLogGroup, error) {
	taskDef, err := resolveTaskDefinition(client, ctx, cluster, service)
	if err != nil {
		return nil, err
	}

	out, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDef,
	})
	if err != nil {
		return nil, fmt.Errorf("describing task definition %s: %w", taskDef, err)
	}

	var groups []cwltypes.ContainerLogGroup
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		if cd.LogConfiguration == nil || cd.LogConfiguration.LogDriver != "awslogs" {
			continue
		}
		opts := cd.LogConfiguration.Options
		if g := opts["awslogs-group"]; g != "" {
			groups = append(groups, cwltypes.ContainerLogGroup{
				Container:    aws.ToString(cd.Name),
				LogGroup:     g,
				StreamPrefix: opts["awslogs-stream-prefix"],
			})
		}
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("no awslogs log groups found for service %s in cluster %s", service, cluster)
	}
	return groups, nil
}

// FindLogGroup resolves a single container's log group. If container is empty,
// returns the first container with awslogs configuration.
func FindLogGroup(client logGroupClient, ctx context.Context, cluster, service, container string) (*cwltypes.ContainerLogGroup, error) {
	groups, err := FindLogGroups(client, ctx, cluster, service)
	if err != nil {
		return nil, err
	}
	if container == "" {
		return &groups[0], nil
	}
	for _, g := range groups {
		if strings.EqualFold(g.Container, container) {
			return &g, nil
		}
	}
	return nil, fmt.Errorf("no awslogs log group found for container %q in service %s", container, service)
}

func resolveTaskDefinition(client logGroupClient, ctx context.Context, cluster, service string) (string, error) {
	out, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return "", fmt.Errorf("describing service %s: %w", service, err)
	}
	if len(out.Services) == 0 {
		return "", fmt.Errorf("service %s not found in cluster %s", service, cluster)
	}
	return aws.ToString(out.Services[0].TaskDefinition), nil
}
