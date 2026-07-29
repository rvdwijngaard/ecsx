// Package envvars provides formatting utilities for container environment variables.
package envvars

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	ecsconnector "github.com/rvdwijngaard/ecsx/pkg/aws/ecs"
	ecstypes "github.com/rvdwijngaard/ecsx/pkg/aws/ecs/types"
)

// Format represents an output format for environment variables.
type Format int

const (
	FormatDetail Format = iota // Styled detail view (like service/task details pane)
	FormatExport               // export NAME="value"
	FormatShell                // NAME=value (quoted if needed)
	FormatDocker               // NAME=value (no quoting, docker env-file)
	FormatJSON                 // {"NAME": "value", ...}
	formatCount                // sentinel
)

func (f Format) String() string {
	switch f {
	case FormatDetail:
		return "detail"
	case FormatExport:
		return "export"
	case FormatShell:
		return "shell"
	case FormatDocker:
		return "docker"
	case FormatJSON:
		return "json"
	default:
		return "unknown"
	}
}

// NextFormat cycles to the next format.
func NextFormat(f Format) Format {
	return (f + 1) % formatCount
}

// ContainerEnvVars holds the formatted env vars for a single container.
type ContainerEnvVars struct {
	Container string
	EnvVars   []ecstypes.EnvVar
	Formatted [formatCount]string // pre-rendered in each format
}

// ResolveContainers returns all container names from the service's task definition.
// Unlike log group resolution, this includes containers without CloudWatch logging.
func ResolveContainers(ecsClient *ecs.Client, ctx context.Context, cluster, service string) ([]string, error) {
	taskDef, err := resolveTaskDef(ecsClient, ctx, cluster, service)
	if err != nil {
		return nil, err
	}

	defs, err := ecsconnector.DescribeTaskDefinition(ecsClient, ctx, taskDef)
	if err != nil {
		return nil, fmt.Errorf("describing task definition: %w", err)
	}

	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names, nil
}

// ResolveEnvVars fetches the task definition for a service and returns env vars per container.
func ResolveEnvVars(ecsClient *ecs.Client, ctx context.Context, cluster, service, container string) ([]ContainerEnvVars, error) {
	taskDef, err := resolveTaskDef(ecsClient, ctx, cluster, service)
	if err != nil {
		return nil, err
	}

	defs, err := ecsconnector.DescribeTaskDefinition(ecsClient, ctx, taskDef)
	if err != nil {
		return nil, fmt.Errorf("describing task definition: %w", err)
	}

	var results []ContainerEnvVars
	for _, def := range defs {
		if container != "" && !strings.EqualFold(def.Name, container) {
			continue
		}
		cev := ContainerEnvVars{
			Container: def.Name,
			EnvVars:   sortEnvVars(def.EnvVars),
		}
		cev.Formatted[FormatDetail] = formatTable(cev.EnvVars)
		cev.Formatted[FormatExport] = formatExport(cev.EnvVars)
		cev.Formatted[FormatShell] = formatShell(cev.EnvVars)
		cev.Formatted[FormatDocker] = formatDocker(cev.EnvVars)
		cev.Formatted[FormatJSON] = formatJSON(cev.EnvVars)
		results = append(results, cev)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("container %q not found in task definition", container)
	}
	return results, nil
}

func resolveTaskDef(ecsClient *ecs.Client, ctx context.Context, cluster, service string) (string, error) {
	out, err := ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return "", fmt.Errorf("describing service %s: %w", service, err)
	}
	if len(out.Services) == 0 {
		return "", fmt.Errorf("service %s not found in cluster %s", service, cluster)
	}
	if out.Services[0].TaskDefinition == nil {
		return "", fmt.Errorf("service %s has no task definition", service)
	}
	return *out.Services[0].TaskDefinition, nil
}

func sortEnvVars(vars []ecstypes.EnvVar) []ecstypes.EnvVar {
	sorted := make([]ecstypes.EnvVar, len(vars))
	copy(sorted, vars)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

func formatTable(vars []ecstypes.EnvVar) string {
	if len(vars) == 0 {
		return "(no environment variables)"
	}
	maxLen := 0
	for _, ev := range vars {
		if len(ev.Name) > maxLen {
			maxLen = len(ev.Name)
		}
	}
	var b strings.Builder
	for i, ev := range vars {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%-*s  %s", maxLen, ev.Name, ev.Value)
	}
	return b.String()
}

func formatExport(vars []ecstypes.EnvVar) string {
	if len(vars) == 0 {
		return "(no environment variables)"
	}
	var b strings.Builder
	for i, ev := range vars {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "export %s=%q", ev.Name, ev.Value)
	}
	return b.String()
}

func formatShell(vars []ecstypes.EnvVar) string {
	if len(vars) == 0 {
		return "(no environment variables)"
	}
	var b strings.Builder
	for i, ev := range vars {
		if i > 0 {
			b.WriteByte('\n')
		}
		// Quote if value contains spaces or special chars
		if strings.ContainsAny(ev.Value, " \t\"'\\$`!#&|;(){}") {
			fmt.Fprintf(&b, "%s=%q", ev.Name, ev.Value)
		} else {
			fmt.Fprintf(&b, "%s=%s", ev.Name, ev.Value)
		}
	}
	return b.String()
}

func formatDocker(vars []ecstypes.EnvVar) string {
	if len(vars) == 0 {
		return "(no environment variables)"
	}
	var b strings.Builder
	for i, ev := range vars {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s=%s", ev.Name, ev.Value)
	}
	return b.String()
}

func formatJSON(vars []ecstypes.EnvVar) string {
	if len(vars) == 0 {
		return "{}"
	}
	m := make(map[string]string, len(vars))
	for _, ev := range vars {
		m[ev.Name] = ev.Value
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}


