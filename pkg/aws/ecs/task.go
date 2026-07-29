package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	apitypes "github.com/rvdwijngaard/ecsx/pkg/aws/ecs/types"
)

type taskClient interface {
	ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	StopTask(context.Context, *ecs.StopTaskInput, ...func(*ecs.Options)) (*ecs.StopTaskOutput, error)
	DescribeTaskDefinition(context.Context, *ecs.DescribeTaskDefinitionInput, ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	ExecuteCommand(context.Context, *ecs.ExecuteCommandInput, ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error)
}

// ListTasks retrieves all tasks for a service in a cluster.
func ListTasks(client taskClient, ctx context.Context, cluster, service string) ([]apitypes.Task, error) {
	arns, err := listAllTaskARNs(client, ctx, cluster, service)
	if err != nil {
		return nil, err
	}
	if len(arns) == 0 {
		return nil, nil
	}
	// DescribeTasks accepts max 100 at a time
	var tasks []apitypes.Task
	for i := 0; i < len(arns); i += 100 {
		end := i + 100
		if end > len(arns) {
			end = len(arns)
		}
		out, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing tasks: %w", err)
		}
		for _, t := range out.Tasks {
			tasks = append(tasks, taskFromAPI(t))
		}
	}
	return tasks, nil
}

// StopTask stops a running task.
func StopTask(client taskClient, ctx context.Context, cluster, taskARN, reason string) error {
	input := &ecs.StopTaskInput{
		Cluster: &cluster,
		Task:    &taskARN,
	}
	if reason != "" {
		input.Reason = &reason
	}
	_, err := client.StopTask(ctx, input)
	if err != nil {
		return fmt.Errorf("stop task: %w", err)
	}
	return nil
}

// DescribeTaskDefinition retrieves container definitions from a task definition.
func DescribeTaskDefinition(client taskClient, ctx context.Context, taskDef string) ([]apitypes.ContainerDefinition, error) {
	out, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDef,
	})
	if err != nil {
		return nil, fmt.Errorf("describing task definition: %w", err)
	}
	var defs []apitypes.ContainerDefinition
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		def := apitypes.ContainerDefinition{
			Name:   aws.ToString(cd.Name),
			Image:  aws.ToString(cd.Image),
			CPU:    int32(cd.Cpu),
			Memory: aws.ToInt32(cd.Memory),
		}
		for _, ev := range cd.Environment {
			def.EnvVars = append(def.EnvVars, apitypes.EnvVar{
				Name:  aws.ToString(ev.Name),
				Value: aws.ToString(ev.Value),
			})
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// ExecuteCommand calls the ECS ExecuteCommand API and returns the session
// details as JSON suitable for session-manager-plugin.
//
// We construct the JSON explicitly to guarantee non-null values for every
// required field. Marshalling the SDK's *ecstypes.Session directly would
// produce `"Field":null` for any nil pointer, which session-manager-plugin
// fails to handle (it asserts fields as concrete types and panics on nil).
func ExecuteCommand(client taskClient, ctx context.Context, cluster, task, container, command string) (*apitypes.ExecuteCommandOutput, error) {
	interactive := true
	out, err := client.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     &cluster,
		Task:        &task,
		Container:   &container,
		Command:     &command,
		Interactive: interactive,
	})
	if err != nil {
		return nil, fmt.Errorf("execute command: %w", err)
	}
	if out.Session == nil {
		return nil, fmt.Errorf("execute command: empty session response")
	}

	sessionJSON, err := json.Marshal(struct {
		SessionId  string `json:"SessionId"`
		TokenValue string `json:"TokenValue"`
		StreamUrl  string `json:"StreamUrl"`
	}{
		SessionId:  aws.ToString(out.Session.SessionId),
		TokenValue: aws.ToString(out.Session.TokenValue),
		StreamUrl:  aws.ToString(out.Session.StreamUrl),
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling session: %w", err)
	}
	if sessionJSON == nil || len(sessionJSON) == 0 ||
		aws.ToString(out.Session.SessionId) == "" ||
		aws.ToString(out.Session.TokenValue) == "" ||
		aws.ToString(out.Session.StreamUrl) == "" {
		return nil, fmt.Errorf("execute command: incomplete session response")
	}
	return &apitypes.ExecuteCommandOutput{
		Session:   sessionJSON,
		TaskARN:   task,
		Container: container,
		Cluster:   cluster,
	}, nil
}

func listAllTaskARNs(client taskClient, ctx context.Context, cluster, service string) ([]string, error) {
	var arns []string
	input := &ecs.ListTasksInput{Cluster: &cluster, ServiceName: &service}
	p := ecs.NewListTasksPaginator(client, input)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing tasks: %w", err)
		}
		arns = append(arns, page.TaskArns...)
	}
	return arns, nil
}

func taskFromAPI(t ecstypes.Task) apitypes.Task {
	task := apitypes.Task{
		ARN:            aws.ToString(t.TaskArn),
		TaskDefinition: aws.ToString(t.TaskDefinitionArn),
		LastStatus:     aws.ToString(t.LastStatus),
		DesiredStatus:  aws.ToString(t.DesiredStatus),
		LaunchType:     string(t.LaunchType),
		HealthStatus:   string(t.HealthStatus),
		Group:          aws.ToString(t.Group),
		CPU:            aws.ToString(t.Cpu),
		Memory:         aws.ToString(t.Memory),
		StartedAt:      t.StartedAt,
		CreatedAt:      t.CreatedAt,
		StoppedAt:      t.StoppedAt,
		StoppedReason:  aws.ToString(t.StoppedReason),
	}
	// Extract task ID from ARN
	if arn := task.ARN; arn != "" {
		parts := strings.Split(arn, "/")
		task.ID = parts[len(parts)-1]
	}
	task.Status = task.LastStatus
	if t.ContainerInstanceArn != nil {
		task.ContainerInstanceID = aws.ToString(t.ContainerInstanceArn)
	}
	// Fargate networking: extract IPs from attachments
	for _, att := range t.Attachments {
		if aws.ToString(att.Type) == "ElasticNetworkInterface" {
			for _, kv := range att.Details {
				switch aws.ToString(kv.Name) {
				case "privateIPv4Address":
					task.PrivateIP = aws.ToString(kv.Value)
				case "publicIPv4Address":
					task.PublicIP = aws.ToString(kv.Value)
				}
			}
		}
	}
	// Containers
	for _, c := range t.Containers {
		container := apitypes.Container{
			Name:         aws.ToString(c.Name),
			Status:       aws.ToString(c.LastStatus),
			Reason:       aws.ToString(c.Reason),
			HealthStatus: string(c.HealthStatus),
			Image:        aws.ToString(c.Image),
		}
		if c.ExitCode != nil {
			container.ExitCode = c.ExitCode
		}
		for _, nb := range c.NetworkBindings {
			container.NetworkBindings = append(container.NetworkBindings, apitypes.NetworkBinding{
				ContainerPort: aws.ToInt32(nb.ContainerPort),
				HostPort:      aws.ToInt32(nb.HostPort),
				Protocol:      string(nb.Protocol),
			})
		}
		task.Containers = append(task.Containers, container)
	}
	return task
}
