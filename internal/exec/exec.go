package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"

	ecsaws "github.com/rvdwijngaard/ecsx/internal/aws"
)

// Connect runs ECS ExecuteCommand and hands off to session-manager-plugin.
func Connect(ctx context.Context, client ecsaws.ECSClient, cluster, service, task, container, command, region, profile string) error {
	// Resolve task from service if not provided
	if task == "" {
		tasks, err := client.ListTasks(ctx, cluster, service)
		if err != nil {
			return fmt.Errorf("listing tasks: %w", err)
		}
		if len(tasks) == 0 {
			return fmt.Errorf("no running tasks for service %s", service)
		}
		task = tasks[0].ARN
		if container == "" && len(tasks[0].Containers) == 1 {
			container = tasks[0].Containers[0].Name
		}
	}
	if container == "" {
		return fmt.Errorf("multiple containers in task, specify one with --container")
	}
	if command == "" {
		command = "/bin/sh"
	}

	out, err := client.ExecuteCommand(ctx, cluster, task, container, command)
	if err != nil {
		return err
	}

	// Build SSM target JSON
	parts := strings.Split(out.TaskARN, "/")
	taskID := parts[len(parts)-1]
	target := struct {
		Target string `json:"Target"`
	}{
		Target: fmt.Sprintf("ecs:%s_%s_%s", cluster, taskID, container),
	}
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("marshalling target: %w", err)
	}

	endpoint := fmt.Sprintf("https://ecs.%s.amazonaws.com", region)

	// session-manager-plugin <session-json> <region> StartSession <profile> <target-json> <endpoint>
	args := []string{
		string(out.Session),
		region,
		"StartSession",
		profile,
		string(targetJSON),
		endpoint,
	}
	cmd := osexec.CommandContext(ctx, "session-manager-plugin", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
