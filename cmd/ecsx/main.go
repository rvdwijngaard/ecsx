package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	ecsaws "github.com/ron/ecsx/internal/aws"
	ecsexec "github.com/ron/ecsx/internal/exec"
	"github.com/ron/ecsx/internal/logs"
	"github.com/ron/ecsx/internal/ssm"
	"github.com/ron/ecsx/internal/ui"
)

var (
	profile string
	region  string
	cluster string
)

func main() {
	root := &cobra.Command{
		Use:   "ecsx",
		Short: "ECS terminal UI and log tailer",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ecsaws.NewCachedClient(profile, region)
			if err != nil {
				return err
			}
			p := tea.NewProgram(ui.New(client, cluster))
			_, err = p.Run()
			return err
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile")
	root.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region")
	root.PersistentFlags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name")
	root.RegisterFlagCompletionFunc("cluster", completeClusters)

	root.AddCommand(logsCmd())
	root.AddCommand(ssmCmd())
	root.AddCommand(execCmd())
	root.AddCommand(completionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func logsCmd() *cobra.Command {
	var service, task, filter string
	var follow, streamName bool
	var start, end logs.TimeFlag
	start.Default(1 * time.Hour)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail ECS service logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ecsaws.NewClient(profile, region)
			if err != nil {
				return err
			}
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()
			return logs.Tail(ctx, client, logs.Options{
				Cluster:    cluster,
				Service:    service,
				Task:       task,
				Filter:     filter,
				Follow:     follow,
				StreamName: streamName,
				Start:      start.Time(),
				End:        end.TimePtr(),
			})
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name (required)")
	cmd.Flags().StringVarP(&task, "task", "t", "", "Filter to specific task ID")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "CloudWatch filter pattern")
	cmd.Flags().BoolVarP(&follow, "follow", "F", true, "Follow log output (use --no-follow to dump and exit)")
	cmd.Flags().BoolVarP(&streamName, "stream-name", "n", false, "Print the log stream name per line")
	cmd.Flags().VarP(&start, "start", "b", "Start time: duration (2h30m) or datetime (2024-01-15T09:00)")
	cmd.Flags().VarP(&end, "end", "e", "End time: duration (30m) or datetime (2024-01-15T10:00)")
	cmd.MarkFlagRequired("service")
	cmd.MarkPersistentFlagRequired("cluster")

	cmd.RegisterFlagCompletionFunc("service", completeWith(func(ctx context.Context, client *ecsaws.Client) ([]string, error) {
		return client.ListServiceNames(ctx, cluster)
	}))
	cmd.RegisterFlagCompletionFunc("task", completeWith(func(ctx context.Context, client *ecsaws.Client) ([]string, error) {
		tasks, err := client.ListTasks(ctx, cluster, service)
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(tasks))
		for i, t := range tasks {
			ids[i] = t.ID
		}
		return ids, nil
	}))

	return cmd
}

func ssmCmd() *cobra.Command {
	var service, instance string

	cmd := &cobra.Command{
		Use:   "ssm",
		Short: "Start an SSM session on an EC2 container instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ecsaws.NewClient(profile, region)
			if err != nil {
				return err
			}
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()
			return ssm.Connect(ctx, client, cluster, service, instance, client.Region(), profile)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&service, "service", "s", "", "ECS service (resolve instance from its tasks)")
	cmd.Flags().StringVarP(&instance, "instance", "i", "", "EC2 instance ID (connect directly)")
	cmd.MarkPersistentFlagRequired("cluster")

	cmd.RegisterFlagCompletionFunc("service", completeWith(func(ctx context.Context, client *ecsaws.Client) ([]string, error) {
		return client.ListServiceNames(ctx, cluster)
	}))

	return cmd
}

func execCmd() *cobra.Command {
	var service, task, container, command string

	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a command in a running ECS container",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ecsaws.NewClient(profile, region)
			if err != nil {
				return err
			}
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()
			return ecsexec.Connect(ctx, client, cluster, service, task, container, command, client.Region(), profile)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name")
	cmd.Flags().StringVarP(&task, "task", "t", "", "Task ARN or ID")
	cmd.Flags().StringVarP(&container, "container", "u", "", "Container name (defaults to first if only one)")
	cmd.Flags().StringVar(&command, "cmd", "/bin/sh", "Command to run")
	cmd.MarkPersistentFlagRequired("cluster")

	cmd.RegisterFlagCompletionFunc("service", completeWith(func(ctx context.Context, client *ecsaws.Client) ([]string, error) {
		return client.ListServiceNames(ctx, cluster)
	}))

	return cmd
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Output shell completion script",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}
