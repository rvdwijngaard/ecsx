package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	apitypes "github.com/ron/ecsx/pkg/aws/ecs/types"
)

type clusterClient interface {
	ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	DescribeClusters(context.Context, *ecs.DescribeClustersInput, ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
}

// ListClusters retrieves all ECS clusters and their details.
func ListClusters(client clusterClient, ctx context.Context) ([]apitypes.Cluster, error) {
	arns, err := listAllClusterARNs(client, ctx)
	if err != nil {
		return nil, err
	}
	if len(arns) == 0 {
		return nil, nil
	}
	out, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: arns,
	})
	if err != nil {
		return nil, fmt.Errorf("describing clusters: %w", err)
	}
	clusters := make([]apitypes.Cluster, 0, len(out.Clusters))
	for _, cl := range out.Clusters {
		clusters = append(clusters, apitypes.Cluster{
			Name:               aws.ToString(cl.ClusterName),
			ARN:                aws.ToString(cl.ClusterArn),
			Status:             aws.ToString(cl.Status),
			ContainerInstances: cl.RegisteredContainerInstancesCount,
			ActiveServices:     cl.ActiveServicesCount,
			RunningTasks:       cl.RunningTasksCount,
			PendingTasks:       cl.PendingTasksCount,
		})
	}
	return clusters, nil
}

func listAllClusterARNs(client clusterClient, ctx context.Context) ([]string, error) {
	var arns []string
	p := ecs.NewListClustersPaginator(client, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}
	return arns, nil
}
