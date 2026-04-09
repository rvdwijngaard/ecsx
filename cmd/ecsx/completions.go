package main

import (
	"context"
	"time"

	ecsaws "github.com/ron/ecsx/internal/aws"
	"github.com/spf13/cobra"
)

// completeWith creates a cobra completion function that fetches string results via the given fn.
func completeWith(fn func(ctx context.Context, client *ecsaws.Client) ([]string, error)) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, err := ecsaws.NewClient(profile, region)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		results, err := fn(ctx, client)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return results, cobra.ShellCompDirectiveNoFileComp
	}
}

var completeClusters = completeWith(func(ctx context.Context, client *ecsaws.Client) ([]string, error) {
	clusters, err := client.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(clusters))
	for i, c := range clusters {
		names[i] = c.Name
	}
	return names, nil
})
