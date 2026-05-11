package ecs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	connector "github.com/ron/ecsx/pkg/aws/ecs"
	apitypes "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs/types"
)

// ListTasks calls the ECS connector and transforms the result for UI display.
func ListTasks(client *ecs.Client, ctx context.Context, cluster, service string) ([]apitypes.TaskItem, error) {
	tasks, err := connector.ListTasks(client, ctx, cluster, service)
	if err != nil {
		return nil, err
	}
	items := make([]apitypes.TaskItem, len(tasks))
	for i, t := range tasks {
		items[i] = apitypes.TaskFromConnector(t)
	}
	return items, nil
}
