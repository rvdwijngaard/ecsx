package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	apitypes "github.com/rvdwijngaard/ecsx/pkg/aws/ecs/types"
)

type instanceClient interface {
	ListContainerInstances(context.Context, *ecs.ListContainerInstancesInput, ...func(*ecs.Options)) (*ecs.ListContainerInstancesOutput, error)
	DescribeContainerInstances(context.Context, *ecs.DescribeContainerInstancesInput, ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error)
}

// ListContainerInstances retrieves all container instances in a cluster.
func ListContainerInstances(client instanceClient, ctx context.Context, cluster string) ([]apitypes.ContainerInstance, error) {
	var arns []string
	p := ecs.NewListContainerInstancesPaginator(client, &ecs.ListContainerInstancesInput{Cluster: &cluster})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing container instances: %w", err)
		}
		arns = append(arns, page.ContainerInstanceArns...)
	}
	if len(arns) == 0 {
		return nil, nil
	}
	// DescribeContainerInstances accepts max 100 at a time
	var instances []apitypes.ContainerInstance
	for i := 0; i < len(arns); i += 100 {
		end := i + 100
		if end > len(arns) {
			end = len(arns)
		}
		out, err := client.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
			Cluster:            &cluster,
			ContainerInstances: arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing container instances: %w", err)
		}
		for _, ci := range out.ContainerInstances {
			instances = append(instances, apitypes.ContainerInstance{
				ARN:           aws.ToString(ci.ContainerInstanceArn),
				EC2InstanceID: aws.ToString(ci.Ec2InstanceId),
				Status:        aws.ToString(ci.Status),
				RunningTasks:  ci.RunningTasksCount,
				PendingTasks:  ci.PendingTasksCount,
			})
		}
	}
	return instances, nil
}

// ResolveTaskEC2Instances maps container instance ARNs to EC2 instance IDs for a set of tasks.
func ResolveTaskEC2Instances(client instanceClient, ctx context.Context, cluster string, tasks []apitypes.Task) (map[string]string, error) {
	// Collect unique container instance ARNs
	ciSet := map[string]bool{}
	for _, t := range tasks {
		if t.ContainerInstanceID != "" {
			ciSet[t.ContainerInstanceID] = true
		}
	}
	if len(ciSet) == 0 {
		return nil, nil
	}
	ciARNs := make([]string, 0, len(ciSet))
	for arn := range ciSet {
		ciARNs = append(ciARNs, arn)
	}
	out, err := client.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
		Cluster:            &cluster,
		ContainerInstances: ciARNs,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(out.ContainerInstances))
	for _, ci := range out.ContainerInstances {
		result[aws.ToString(ci.ContainerInstanceArn)] = aws.ToString(ci.Ec2InstanceId)
	}
	return result, nil
}
