package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	apitypes "github.com/rvdwijngaard/ecsx/pkg/aws/ecs/types"
)

type serviceClient interface {
	ListServices(context.Context, *ecs.ListServicesInput, ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(context.Context, *ecs.DescribeServicesInput, ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	UpdateService(context.Context, *ecs.UpdateServiceInput, ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
}

// ListServices retrieves all services in a cluster with full details.
func ListServices(client serviceClient, ctx context.Context, cluster string) ([]apitypes.Service, error) {
	arns, err := listAllServiceARNs(client, ctx, cluster)
	if err != nil {
		return nil, err
	}
	if len(arns) == 0 {
		return nil, nil
	}
	// DescribeServices accepts max 10 at a time
	var services []apitypes.Service
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		out, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing services: %w", err)
		}
		for _, s := range out.Services {
			services = append(services, serviceFromAPI(s))
		}
	}
	return services, nil
}

// ListServiceNames retrieves just the service names in a cluster.
func ListServiceNames(client serviceClient, ctx context.Context, cluster string) ([]string, error) {
	arns, err := listAllServiceARNs(client, ctx, cluster)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(arns))
	for i, arn := range arns {
		parts := strings.Split(arn, "/")
		names[i] = parts[len(parts)-1]
	}
	return names, nil
}

// DescribeService retrieves details for a single service.
func DescribeService(client serviceClient, ctx context.Context, cluster, service string) (*apitypes.Service, error) {
	out, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", service)
	}
	svc := serviceFromAPI(out.Services[0])
	return &svc, nil
}

// UpdateServiceDesiredCount sets the desired task count for a service.
func UpdateServiceDesiredCount(client serviceClient, ctx context.Context, cluster, service string, desiredCount int32) error {
	_, err := client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &cluster,
		Service:      &service,
		DesiredCount: &desiredCount,
	})
	if err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	return nil
}

func listAllServiceARNs(client serviceClient, ctx context.Context, cluster string) ([]string, error) {
	var arns []string
	p := ecs.NewListServicesPaginator(client, &ecs.ListServicesInput{Cluster: &cluster})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing services: %w", err)
		}
		arns = append(arns, page.ServiceArns...)
	}
	return arns, nil
}

func serviceFromAPI(s ecstypes.Service) apitypes.Service {
	lt := ""
	if s.LaunchType != "" {
		lt = string(s.LaunchType)
	}
	return apitypes.Service{
		Name:           aws.ToString(s.ServiceName),
		ARN:            aws.ToString(s.ServiceArn),
		Status:         aws.ToString(s.Status),
		TaskDefinition: aws.ToString(s.TaskDefinition),
		DesiredCount:   s.DesiredCount,
		RunningCount:   s.RunningCount,
		PendingCount:   s.PendingCount,
		LaunchType:     lt,
		CreatedAt:      s.CreatedAt,
	}
}
