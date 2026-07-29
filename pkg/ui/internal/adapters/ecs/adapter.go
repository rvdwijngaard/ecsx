// Package ecs adapts ECS connector responses for UI display.
package ecs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	connector "github.com/rvdwijngaard/ecsx/pkg/aws/ecs"
	apitypes "github.com/rvdwijngaard/ecsx/pkg/ui/internal/adapters/ecs/types"
)

// ListClusters calls the ECS connector and transforms the result for UI display.
func ListClusters(client *ecs.Client, ctx context.Context) ([]apitypes.ClusterItem, error) {
	clusters, err := connector.ListClusters(client, ctx)
	if err != nil {
		return nil, err
	}
	items := make([]apitypes.ClusterItem, len(clusters))
	for i, c := range clusters {
		items[i] = apitypes.ClusterFromConnector(c)
	}
	return items, nil
}

// DescribeCluster returns a single cluster's details for UI display.
// It finds the cluster by name from the full list.
func DescribeCluster(client *ecs.Client, ctx context.Context, clusterName string) (*apitypes.ClusterItem, error) {
	clusters, err := connector.ListClusters(client, ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.Name == clusterName {
			item := apitypes.ClusterFromConnector(c)
			return &item, nil
		}
	}
	return nil, nil
}
