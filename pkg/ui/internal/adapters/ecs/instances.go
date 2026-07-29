package ecs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	connector "github.com/rvdwijngaard/ecsx/pkg/aws/ecs"
)

// ListClusterInstanceIDs returns the unique EC2 instance IDs of all container
// instances registered to a cluster. Returns an empty slice (not an error) if
// the cluster has no instances.
func ListClusterInstanceIDs(client *ecs.Client, ctx context.Context, cluster string) ([]string, error) {
	instances, err := connector.ListContainerInstances(client, ctx, cluster)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(instances))
	ids := make([]string, 0, len(instances))
	for _, i := range instances {
		if i.EC2InstanceID == "" || seen[i.EC2InstanceID] {
			continue
		}
		seen[i.EC2InstanceID] = true
		ids = append(ids, i.EC2InstanceID)
	}
	return ids, nil
}

// ListServiceInstanceIDs returns the unique EC2 instance IDs of the EC2
// instances running tasks for a given service. Returns an empty slice if the
// service has no tasks or runs entirely on Fargate.
func ListServiceInstanceIDs(client *ecs.Client, ctx context.Context, cluster, service string) ([]string, error) {
	tasks, err := connector.ListTasks(client, ctx, cluster, service)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	ec2Map, err := connector.ResolveTaskEC2Instances(client, ctx, cluster, tasks)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(ec2Map))
	ids := make([]string, 0, len(ec2Map))
	for _, id := range ec2Map {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}