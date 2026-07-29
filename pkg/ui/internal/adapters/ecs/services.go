package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	connector "github.com/rvdwijngaard/ecsx/pkg/aws/ecs"
	apitypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs/types"
)

// ListServiceNames calls the ECS connector and returns service names for a cluster.
func ListServiceNames(client *ecs.Client, ctx context.Context, cluster string) ([]string, error) {
	return connector.ListServiceNames(client, ctx, cluster)
}

// ListServices calls the ECS connector and transforms the result for UI display.
func ListServices(client *ecs.Client, ctx context.Context, cluster string) ([]apitypes.ServiceItem, error) {
	services, err := connector.ListServices(client, ctx, cluster)
	if err != nil {
		return nil, err
	}
	items := make([]apitypes.ServiceItem, len(services))
	for i, s := range services {
		items[i] = apitypes.ServiceItem{
			Name:           s.Name,
			ARN:            s.ARN,
			Status:         s.Status,
			TaskDefinition: s.TaskDefinition,
			DesiredCount:   s.DesiredCount,
			RunningCount:   s.RunningCount,
			PendingCount:   s.PendingCount,
			LaunchType:     s.LaunchType,
			CreatedAt:      s.CreatedAt,
		}
	}
	return items, nil
}

// DescribeService returns a formatted description string for a service.
func DescribeService(s apitypes.ServiceItem) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "Service:     %s\n", s.Name)
	fmt.Fprintf(&b, "Status:      %s\n", s.Status)
	fmt.Fprintf(&b, "Launch Type: %s\n", s.LaunchType)
	fmt.Fprintf(&b, "Task Def:    %s\n", s.TaskDefinition)
	fmt.Fprintf(&b, "Desired:     %d\n", s.DesiredCount)
	fmt.Fprintf(&b, "Running:     %d\n", s.RunningCount)
	fmt.Fprintf(&b, "Pending:     %d\n", s.PendingCount)
	if s.CreatedAt != nil {
		fmt.Fprintf(&b, "Created:     %s\n", s.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	return b.String()
}
