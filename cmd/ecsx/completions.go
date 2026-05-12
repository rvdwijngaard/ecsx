package main

// import (
// 	"context"
// 	"time"
//
// 	"github.com/aws/aws-sdk-go-v2/service/ecs"
// 	"github.com/spf13/cobra"
//
// 	pkgaws "github.com/ron/ecsx/pkg/aws"
// 	ecsconnector "github.com/ron/ecsx/pkg/aws/ecs"
// )
//
// // completeWith creates a cobra completion function that fetches string results via the given fn.
// func completeWith(fn func(ctx context.Context, client *ecs.Client) ([]string, error)) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
// 	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
// 		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 		defer cancel()
//
// 		awsCfg, err := pkgaws.LoadAWSConfig(ctx, resolveRegion(), resolveProfile(), nil)
// 		if err != nil {
// 			return nil, cobra.ShellCompDirectiveError
// 		}
// 		profileStr := ""
// 		if p := resolveProfile(); p != nil {
// 			profileStr = *p
// 		}
// 		ecsClient := ecsconnector.NewClient(awsCfg, profileStr)
//
// 		results, err := fn(ctx, ecsClient.ECS)
// 		if err != nil {
// 			return nil, cobra.ShellCompDirectiveError
// 		}
// 		return results, cobra.ShellCompDirectiveNoFileComp
// 	}
// }
//
// var completeClusters = completeWith(func(ctx context.Context, client *ecs.Client) ([]string, error) {
// 	clusters, err := ecsconnector.ListClusters(client, ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	names := make([]string, len(clusters))
// 	for i, c := range clusters {
// 		names[i] = c.Name
// 	}
// 	return names, nil
// })
