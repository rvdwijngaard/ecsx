package ecs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	connector "github.com/rvdwijngaard/ecsx/pkg/aws/ecs"
	apitypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs/types"
)

// ListTasks calls the ECS connector and transforms the result for UI display.
func ListTasks(client *ecs.Client, ctx context.Context, cluster, service, region string) ([]apitypes.TaskItem, error) {
	tasks, err := connector.ListTasks(client, ctx, cluster, service)
	if err != nil {
		return nil, err
	}

	// Resolve EC2 instance IDs for EC2 launch type tasks
	ec2Map, _ := connector.ResolveTaskEC2Instances(client, ctx, cluster, tasks)

	items := make([]apitypes.TaskItem, len(tasks))
	for i, t := range tasks {
		items[i] = apitypes.TaskFromConnector(t)
		items[i].ClusterName = cluster
		items[i].Region = region
		if ec2Map != nil && t.ContainerInstanceID != "" {
			items[i].EC2InstanceID = ec2Map[t.ContainerInstanceID]
		}
	}
	return items, nil
}
