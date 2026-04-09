package ssm

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	ecsaws "github.com/ron/ecsx/internal/aws"
)

// Connect starts an SSM session. It resolves the target EC2 instance from
// the provided instance ID, service name, or cluster container instances.
func Connect(ctx context.Context, client ecsaws.ECSClient, cluster, service, instance, region, profile string) error {
	if instance == "" {
		var err error
		instance, err = resolveInstance(ctx, client, cluster, service)
		if err != nil {
			return err
		}
	}

	args := []string{"ssm", "start-session", "--target", instance, "--region", region}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveInstance(ctx context.Context, client ecsaws.ECSClient, cluster, service string) (string, error) {
	if service != "" {
		tasks, err := client.ListTasks(ctx, cluster, service)
		if err != nil {
			return "", fmt.Errorf("listing tasks: %w", err)
		}
		for _, t := range tasks {
			if t.EC2InstanceID != "" {
				return t.EC2InstanceID, nil
			}
		}
		return "", fmt.Errorf("no EC2-backed tasks found for service %s", service)
	}

	instances, err := client.ListContainerInstances(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("listing container instances: %w", err)
	}
	if len(instances) == 0 {
		return "", fmt.Errorf("no container instances in cluster %s", cluster)
	}
	return instances[0].EC2InstanceID, nil
}
