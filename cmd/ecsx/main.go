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
	"github.com/ron/ecsx/internal/debug"
	ecsexec "github.com/ron/ecsx/internal/exec"
	"github.com/ron/ecsx/internal/logs"
	"github.com/ron/ecsx/internal/ssm"
	"github.com/ron/ecsx/internal/ui"
)

var (
	version string
	profile string
	region  string
	cluster string
	verbose bool
)

func main() {
	root := &cobra.Command{
		Use:     "ecsx",
		Short:   "ECS terminal UI and log tailer",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				debug.Enable()
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			debug.Log("creating AWS client (profile=%q region=%q)", profile, region)
			client, err := ecsaws.NewCachedClient(profile, region)
			if err != nil {
				return err
			}
			debug.Log("AWS client created, starting TUI (cluster=%q)", cluster)
			p := tea.NewProgram(ui.New(client, cluster))
			_, err = p.Run()
			return err
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile")
	root.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region")
	root.PersistentFlags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	root.RegisterFlagCompletionFunc("cluster", completeClusters)

	root.AddCommand(logsCmd())
	root.AddCommand(ssmCmd())
	root.AddCommand(execCmd())
	root.AddCommand(taskCmd())
	root.AddCommand(containerEnvCmd())
	root.AddCommand(completionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func logsCmd() *cobra.Command {
	var service, task, filter string
	var follow, streamName, groupName, timestamp, eventID bool
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
				GroupName:  groupName,
				Timestamp:  timestamp,
				EventID:    eventID,
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
	cmd.Flags().BoolVarP(&groupName, "group-name", "g", false, "Print the log group name per line")
	cmd.Flags().BoolVarP(&timestamp, "timestamp", "T", false, "Print the event timestamp")
	cmd.Flags().BoolVarP(&eventID, "event-id", "i", false, "Print the event ID")
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

func taskCmd() *cobra.Command {
	var service, task string

	cmd := &cobra.Command{
		Use:   "task",
		Short: "Describe a task (or an arbitrary task from a service)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == "" && task == "" {
				return fmt.Errorf("provide --service or --task")
			}
			client, err := ecsaws.NewClient(profile, region)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			var tasks []ecsaws.Task
			if service != "" {
				tasks, err = client.ListTasks(ctx, cluster, service)
				if err != nil {
					return err
				}
				if task != "" {
					for _, t := range tasks {
						if t.ID == task || t.ARN == task {
							tasks = []ecsaws.Task{t}
							break
						}
					}
				}
			}
			if len(tasks) == 0 {
				return fmt.Errorf("no tasks found")
			}

			t := tasks[0]
			fmt.Printf("Task:            %s\n", t.ID)
			fmt.Printf("ARN:             %s\n", t.ARN)
			fmt.Printf("Status:          %s\n", t.Status)
			fmt.Printf("Desired Status:  %s\n", t.DesiredStatus)
			fmt.Printf("Health:          %s\n", t.HealthStatus)
			fmt.Printf("Launch Type:     %s\n", t.LaunchType)
			fmt.Printf("Task Definition: %s\n", t.TaskDefinition)
			if t.CPU != "" {
				fmt.Printf("CPU / Memory:    %s / %s\n", t.CPU, t.Memory)
			}
			if t.StartedAt != nil {
				fmt.Printf("Started At:      %s\n", t.StartedAt.Local().Format(time.RFC3339))
			}
			if t.StoppedAt != nil {
				fmt.Printf("Stopped At:      %s\n", t.StoppedAt.Local().Format(time.RFC3339))
			}
			if t.StoppedReason != "" {
				fmt.Printf("Stopped Reason:  %s\n", t.StoppedReason)
			}
			if t.EC2InstanceID != "" {
				fmt.Printf("EC2 Instance:    %s\n", t.EC2InstanceID)
			}
			if t.PrivateIP != "" {
				fmt.Printf("Private IP:      %s\n", t.PrivateIP)
			}
			if t.PublicIP != "" {
				fmt.Printf("Public IP:       %s\n", t.PublicIP)
			}
			for _, c := range t.Containers {
				fmt.Printf("\nContainer: %s\n", c.Name)
				fmt.Printf("  Status: %s\n", c.Status)
				fmt.Printf("  Image:  %s\n", c.Image)
				if c.HealthStatus != "" && c.HealthStatus != "UNKNOWN" {
					fmt.Printf("  Health: %s\n", c.HealthStatus)
				}
				if c.ExitCode != nil {
					fmt.Printf("  Exit:   %d\n", *c.ExitCode)
				}
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name")
	cmd.Flags().StringVarP(&task, "task", "t", "", "Task ID or ARN")
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

func containerEnvCmd() *cobra.Command {
	var service, container, format string

	cmd := &cobra.Command{
		Use:   "container-env",
		Short: "List environment variables for a service's containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ecsaws.NewClient(profile, region)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			svc, err := client.DescribeService(ctx, cluster, service)
			if err != nil {
				return err
			}
			defs, err := client.DescribeTaskDefinition(ctx, svc.TaskDefinition)
			if err != nil {
				return err
			}

			for _, cd := range defs {
				if container != "" && cd.Name != container {
					continue
				}
				switch format {
				case "export", "shell":
					for _, ev := range cd.EnvVars {
						fmt.Printf("export %s=%q\n", ev.Name, ev.Value)
					}
				case "docker":
					for _, ev := range cd.EnvVars {
						fmt.Printf("-e %s=%s\n", ev.Name, ev.Value)
					}
				default: // table
					if len(defs) > 1 {
						fmt.Printf("# %s\n", cd.Name)
					}
					maxLen := 0
					for _, ev := range cd.EnvVars {
						if len(ev.Name) > maxLen {
							maxLen = len(ev.Name)
						}
					}
					for _, ev := range cd.EnvVars {
						fmt.Printf("%-*s  %s\n", maxLen, ev.Name, ev.Value)
					}
					if len(defs) > 1 {
						fmt.Println()
					}
				}
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name (required)")
	cmd.Flags().StringVar(&container, "container", "", "Filter to a specific container")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, export, shell, docker")
	cmd.MarkFlagRequired("service")
	cmd.MarkPersistentFlagRequired("cluster")
	cmd.RegisterFlagCompletionFunc("service", completeWith(func(ctx context.Context, client *ecsaws.Client) ([]string, error) {
		return client.ListServiceNames(ctx, cluster)
	}))
	cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "export", "shell", "docker"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.RegisterFlagCompletionFunc("container", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if cluster == "" || service == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		client, err := ecsaws.NewClient(profile, region)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		svc, err := client.DescribeService(ctx, cluster, service)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		defs, err := client.DescribeTaskDefinition(ctx, svc.TaskDefinition)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		names := make([]string, len(defs))
		for i, d := range defs {
			names[i] = d.Name
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})
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
