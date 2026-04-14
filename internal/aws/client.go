package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/ron/ecsx/internal/debug"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	logstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type Cluster struct {
	Name               string
	ARN                string
	Status             string
	ContainerInstances int32
	ActiveServices     int32
	RunningTasks       int32
	PendingTasks       int32
}

type ECSClient interface {
	Region() string
	Profile() string
	ListClusters(ctx context.Context) ([]Cluster, error)
	ListServiceNames(ctx context.Context, cluster string) ([]string, error)
	DescribeService(ctx context.Context, cluster, service string) (*Service, error)
	ListServices(ctx context.Context, cluster string) ([]Service, error)
	ListTasks(ctx context.Context, cluster, service string) ([]Task, error)
	ListContainerInstances(ctx context.Context, cluster string) ([]ContainerInstance, error)
	DescribeTaskDefinition(ctx context.Context, taskDef string) ([]ContainerDefinition, error)
	UpdateServiceDesiredCount(ctx context.Context, cluster, service string, desiredCount int32) error
	ForceNewDeployment(ctx context.Context, cluster, service string) error
	StopTask(ctx context.Context, cluster, taskARN, reason string) error
	ExecuteCommand(ctx context.Context, cluster, task, container, command string) (*ExecuteCommandOutput, error)
	GetServiceMetrics(ctx context.Context, cluster, service string) (*ServiceMetrics, error)
	TailLogs(ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string) (<-chan LogEvent, error)
	FetchRecentLogs(ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string, start time.Time, end *time.Time) ([]LogEvent, error)
	ResolveTaskEC2Instances(ctx context.Context, cluster string, tasks []Task) (map[string]string, error)
}

type ContainerInstance struct {
	ARN           string
	EC2InstanceID string
	Status        string
	RunningTasks  int32
	PendingTasks  int32
}

type Client struct {
	ecs     *ecs.Client
	cw      *cw.Client
	logs    *cloudwatchlogs.Client
	region  string
	profile string
}

func NewClient(profile, region string) (*Client, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	debug.Log("AWS config loaded (region=%s)", cfg.Region)
	return &Client{
		ecs:     ecs.NewFromConfig(cfg),
		cw:      cw.NewFromConfig(cfg),
		logs:    cloudwatchlogs.NewFromConfig(cfg),
		region:  cfg.Region,
		profile: profile,
	}, nil
}

func (c *Client) Region() string  { return c.region }
func (c *Client) Profile() string { return c.profile }

func (c *Client) ListClusters(ctx context.Context) ([]Cluster, error) {
	debug.Log("ListClusters: listing cluster ARNs...")
	arns, err := c.listAllClusterARNs(ctx)
	if err != nil {
		return nil, err
	}
	debug.Log("ListClusters: found %d clusters, describing...", len(arns))
	if len(arns) == 0 {
		return nil, nil
	}
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: arns,
	})
	if err != nil {
		return nil, fmt.Errorf("describing clusters: %w", err)
	}
	clusters := make([]Cluster, 0, len(out.Clusters))
	for _, cl := range out.Clusters {
		clusters = append(clusters, clusterFromAPI(cl))
	}
	return clusters, nil
}

func (c *Client) listAllClusterARNs(ctx context.Context) ([]string, error) {
	var arns []string
	p := ecs.NewListClustersPaginator(c.ecs, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}
	return arns, nil
}

func clusterFromAPI(cl types.Cluster) Cluster {
	return Cluster{
		Name:               aws.ToString(cl.ClusterName),
		ARN:                aws.ToString(cl.ClusterArn),
		Status:             aws.ToString(cl.Status),
		ContainerInstances: cl.RegisteredContainerInstancesCount,
		ActiveServices:     cl.ActiveServicesCount,
		RunningTasks:       cl.RunningTasksCount,
		PendingTasks:       cl.PendingTasksCount,
	}
}

func (c *Client) ListContainerInstances(ctx context.Context, cluster string) ([]ContainerInstance, error) {
	var arns []string
	p := ecs.NewListContainerInstancesPaginator(c.ecs, &ecs.ListContainerInstancesInput{Cluster: &cluster})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing container instances: %w", err)
		}
		arns = append(arns, page.ContainerInstanceArns...)
	}
	if len(arns) == 0 {
		return nil, nil
	}
	// DescribeContainerInstances accepts max 100 at a time
	var instances []ContainerInstance
	for i := 0; i < len(arns); i += 100 {
		end := i + 100
		if end > len(arns) {
			end = len(arns)
		}
		out, err := c.ecs.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
			Cluster:            &cluster,
			ContainerInstances: arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing container instances: %w", err)
		}
		for _, ci := range out.ContainerInstances {
			instances = append(instances, ContainerInstance{
				ARN:           aws.ToString(ci.ContainerInstanceArn),
				EC2InstanceID: aws.ToString(ci.Ec2InstanceId),
				Status:        aws.ToString(ci.Status),
				RunningTasks:  ci.RunningTasksCount,
				PendingTasks:  ci.PendingTasksCount,
			})
		}
	}
	return instances, nil
}

type Service struct {
	Name           string
	ARN            string
	Status         string
	TaskDefinition string
	DesiredCount   int32
	RunningCount   int32
	PendingCount   int32
	LaunchType     string
	CreatedAt      *time.Time
}

func (c *Client) ListServices(ctx context.Context, cluster string) ([]Service, error) {
	arns, err := c.listAllServiceARNs(ctx, cluster)
	if err != nil {
		return nil, err
	}
	if len(arns) == 0 {
		return nil, nil
	}
	// DescribeServices accepts max 10 at a time
	var services []Service
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing services: %w", err)
		}
		for _, s := range out.Services {
			services = append(services, serviceFromAPI(s))
		}
	}
	return services, nil
}

func (c *Client) listAllServiceARNs(ctx context.Context, cluster string) ([]string, error) {
	var arns []string
	p := ecs.NewListServicesPaginator(c.ecs, &ecs.ListServicesInput{Cluster: &cluster})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing services: %w", err)
		}
		arns = append(arns, page.ServiceArns...)
	}
	return arns, nil
}

func (c *Client) ListServiceNames(ctx context.Context, cluster string) ([]string, error) {
	arns, err := c.listAllServiceARNs(ctx, cluster)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(arns))
	for i, arn := range arns {
		parts := strings.Split(arn, "/")
		names[i] = parts[len(parts)-1]
	}
	return names, nil
}

func (c *Client) DescribeService(ctx context.Context, cluster, service string) (*Service, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", service)
	}
	svc := serviceFromAPI(out.Services[0])
	return &svc, nil
}

func serviceFromAPI(s types.Service) Service {
	lt := ""
	if s.LaunchType != "" {
		lt = string(s.LaunchType)
	}
	return Service{
		Name:           aws.ToString(s.ServiceName),
		ARN:            aws.ToString(s.ServiceArn),
		Status:         aws.ToString(s.Status),
		TaskDefinition: aws.ToString(s.TaskDefinition),
		DesiredCount:   s.DesiredCount,
		RunningCount:   s.RunningCount,
		PendingCount:   s.PendingCount,
		LaunchType:     lt,
		CreatedAt:      s.CreatedAt,
	}
}

type Container struct {
	Name            string
	Status          string
	ExitCode        *int32
	Reason          string
	HealthStatus    string
	Image           string
	NetworkBindings []NetworkBinding
}

type NetworkBinding struct {
	ContainerPort int32
	HostPort      int32
	Protocol      string
	BindIP        string
}

type Task struct {
	ID                  string
	ARN                 string
	TaskDefinition      string
	Status              string
	LastStatus          string
	DesiredStatus       string
	LaunchType          string
	HealthStatus        string
	Group               string
	CPU                 string
	Memory              string
	StartedAt           *time.Time
	CreatedAt           *time.Time
	StoppedAt           *time.Time
	StoppedReason       string
	ContainerInstanceID string
	EC2InstanceID       string
	Containers          []Container
	// Fargate networking
	PrivateIP string
	PublicIP  string
}

func (c *Client) ListTasks(ctx context.Context, cluster, service string) ([]Task, error) {
	arns, err := c.listAllTaskARNs(ctx, cluster, service)
	if err != nil {
		return nil, err
	}
	if len(arns) == 0 {
		return nil, nil
	}
	// DescribeTasks accepts max 100 at a time
	var tasks []Task
	for i := 0; i < len(arns); i += 100 {
		end := i + 100
		if end > len(arns) {
			end = len(arns)
		}
		out, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
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

func (c *Client) listAllTaskARNs(ctx context.Context, cluster, service string) ([]string, error) {
	var arns []string
	input := &ecs.ListTasksInput{Cluster: &cluster, ServiceName: &service}
	p := ecs.NewListTasksPaginator(c.ecs, input)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing tasks: %w", err)
		}
		arns = append(arns, page.TaskArns...)
	}
	return arns, nil
}

func (c *Client) ResolveTaskEC2Instances(ctx context.Context, cluster string, tasks []Task) (map[string]string, error) {
	return c.resolveContainerInstances(ctx, cluster, tasks)
}

func (c *Client) resolveContainerInstances(ctx context.Context, cluster string, tasks []Task) (map[string]string, error) {
	// Collect unique container instance ARNs
	ciSet := map[string]bool{}
	for _, t := range tasks {
		if t.ContainerInstanceID != "" {
			ciSet[t.ContainerInstanceID] = true
		}
	}
	if len(ciSet) == 0 {
		return nil, nil
	}
	ciARNs := make([]string, 0, len(ciSet))
	for arn := range ciSet {
		ciARNs = append(ciARNs, arn)
	}
	out, err := c.ecs.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
		Cluster:            &cluster,
		ContainerInstances: ciARNs,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(out.ContainerInstances))
	for _, ci := range out.ContainerInstances {
		result[aws.ToString(ci.ContainerInstanceArn)] = aws.ToString(ci.Ec2InstanceId)
	}
	return result, nil
}

func taskFromAPI(t types.Task) Task {
	task := Task{
		ARN:           aws.ToString(t.TaskArn),
		TaskDefinition: aws.ToString(t.TaskDefinitionArn),
		LastStatus:    aws.ToString(t.LastStatus),
		DesiredStatus: aws.ToString(t.DesiredStatus),
		LaunchType:    string(t.LaunchType),
		HealthStatus:  string(t.HealthStatus),
		Group:         aws.ToString(t.Group),
		CPU:           aws.ToString(t.Cpu),
		Memory:        aws.ToString(t.Memory),
		StartedAt:     t.StartedAt,
		CreatedAt:     t.CreatedAt,
		StoppedAt:     t.StoppedAt,
		StoppedReason: aws.ToString(t.StoppedReason),
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
		container := Container{
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
			container.NetworkBindings = append(container.NetworkBindings, NetworkBinding{
				ContainerPort: aws.ToInt32(nb.ContainerPort),
				HostPort:      aws.ToInt32(nb.HostPort),
				Protocol:      string(nb.Protocol),
			})
		}
		task.Containers = append(task.Containers, container)
	}
	return task
}

type EnvVar struct {
	Name  string
	Value string
}

type ContainerDefinition struct {
	Name    string
	Image   string
	CPU     int32
	Memory  int32
	EnvVars []EnvVar
}

func (c *Client) DescribeTaskDefinition(ctx context.Context, taskDef string) ([]ContainerDefinition, error) {
	out, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDef,
	})
	if err != nil {
		return nil, fmt.Errorf("describing task definition: %w", err)
	}
	var defs []ContainerDefinition
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		def := ContainerDefinition{
			Name:   aws.ToString(cd.Name),
			Image:  aws.ToString(cd.Image),
			CPU:    int32(cd.Cpu),
			Memory: aws.ToInt32(cd.Memory),
		}
		for _, ev := range cd.Environment {
			def.EnvVars = append(def.EnvVars, EnvVar{
				Name:  aws.ToString(ev.Name),
				Value: aws.ToString(ev.Value),
			})
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func (c *Client) UpdateServiceDesiredCount(ctx context.Context, cluster, service string, desiredCount int32) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &cluster,
		Service:      &service,
		DesiredCount: &desiredCount,
	})
	if err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	return nil
}

func (c *Client) ForceNewDeployment(ctx context.Context, cluster, service string) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            &cluster,
		Service:            &service,
		ForceNewDeployment: true,
	})
	if err != nil {
		return fmt.Errorf("force new deployment: %w", err)
	}
	return nil
}

func (c *Client) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	input := &ecs.StopTaskInput{
		Cluster: &cluster,
		Task:    &taskARN,
	}
	if reason != "" {
		input.Reason = &reason
	}
	_, err := c.ecs.StopTask(ctx, input)
	if err != nil {
		return fmt.Errorf("stop task: %w", err)
	}
	return nil
}

// ExecuteCommandOutput holds the session info needed to start session-manager-plugin.
type ExecuteCommandOutput struct {
	Session   json.RawMessage
	TaskARN   string
	Container string
	Cluster   string
}

// ExecuteCommand calls the ECS ExecuteCommand API and returns the session details.
func (c *Client) ExecuteCommand(ctx context.Context, cluster, task, container, command string) (*ExecuteCommandOutput, error) {
	interactive := true
	out, err := c.ecs.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     &cluster,
		Task:        &task,
		Container:   &container,
		Command:     &command,
		Interactive: interactive,
	})
	if err != nil {
		return nil, fmt.Errorf("execute command: %w", err)
	}
	sess, err := json.Marshal(out.Session)
	if err != nil {
		return nil, fmt.Errorf("marshalling session: %w", err)
	}
	return &ExecuteCommandOutput{
		Session:   sess,
		TaskARN:   task,
		Container: container,
		Cluster:   cluster,
	}, nil
}

// LogEvent represents a single CloudWatch log event.
type LogEvent struct {
	Timestamp time.Time
	Message   string
	Stream    string
	EventID   string
	LogGroup  string
}

// TailLogs starts a CloudWatch Live Tail session for the given log group.
// Cancel the context to stop the session. Filter pattern uses CloudWatch filter syntax.
func (c *Client) TailLogs(ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string) (<-chan LogEvent, error) {
	// StartLiveTail requires the full log group ARN
	descOut, err := c.logs.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: &logGroup,
	})
	if err != nil {
		return nil, fmt.Errorf("describing log group: %w", err)
	}
	var logGroupARN string
	for _, lg := range descOut.LogGroups {
		if aws.ToString(lg.LogGroupName) == logGroup {
			logGroupARN = aws.ToString(lg.Arn)
			break
		}
	}
	if logGroupARN == "" {
		return nil, fmt.Errorf("log group %s not found", logGroup)
	}
	// Some APIs return ARN with trailing :*, strip it
	logGroupARN = strings.TrimSuffix(logGroupARN, ":*")

	input := &cloudwatchlogs.StartLiveTailInput{
		LogGroupIdentifiers: []string{logGroupARN},
	}
	if logStreamPrefix != "" {
		input.LogStreamNamePrefixes = []string{logStreamPrefix}
	}
	if filterPattern != "" {
		input.LogEventFilterPattern = &filterPattern
	}

	resp, err := c.logs.StartLiveTail(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("starting live tail (group=%s, prefix=%q, filter=%q): %w", logGroupARN, logStreamPrefix, filterPattern, err)
	}

	ch := make(chan LogEvent, 500)
	stream := resp.GetStream()

	go func() {
		defer close(ch)
		defer stream.Close()
		events := stream.Events()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				switch e := event.(type) {
				case *logstypes.StartLiveTailResponseStreamMemberSessionUpdate:
					for _, r := range e.Value.SessionResults {
						ts := time.UnixMilli(*r.Timestamp)
						select {
						case ch <- LogEvent{
							Timestamp: ts,
							Message:   strings.TrimRight(*r.Message, "\n"),
							Stream:    *r.LogStreamName,
							LogGroup:  aws.ToString(r.LogGroupIdentifier),
						}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()
	return ch, nil
}

// FetchRecentLogs retrieves historical log events from the given log group.
func (c *Client) FetchRecentLogs(ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string, start time.Time, end *time.Time) ([]LogEvent, error) {
	startMs := start.UnixMilli()
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &logGroup,
		StartTime:    &startMs,
		Limit:        aws.Int32(200),
	}
	if end != nil {
		endMs := end.UnixMilli()
		input.EndTime = &endMs
	}
	if logStreamPrefix != "" {
		input.LogStreamNamePrefix = &logStreamPrefix
	}
	if filterPattern != "" {
		input.FilterPattern = &filterPattern
	}
	out, err := c.logs.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("filtering log events: %w", err)
	}
	events := make([]LogEvent, 0, len(out.Events))
	for _, e := range out.Events {
		events = append(events, LogEvent{
			Timestamp: time.UnixMilli(aws.ToInt64(e.Timestamp)),
			Message:   strings.TrimRight(aws.ToString(e.Message), "\n"),
			Stream:    aws.ToString(e.LogStreamName),
			EventID:   aws.ToString(e.EventId),
			LogGroup:  logGroup,
		})
	}
	return events, nil
}

// LogGroupForService returns the most common log group pattern for an ECS service.
func (c *Client) LogGroupForService(ctx context.Context, taskDef string) (string, string, error) {
	out, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDef,
	})
	if err != nil {
		return "", "", fmt.Errorf("describing task definition: %w", err)
	}
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		if cd.LogConfiguration != nil && cd.LogConfiguration.LogDriver == "awslogs" {
			opts := cd.LogConfiguration.Options
			group := opts["awslogs-group"]
			prefix := opts["awslogs-stream-prefix"]
			if group != "" {
				return group, prefix, nil
			}
		}
	}
	return "", "", fmt.Errorf("no awslogs log group found in task definition %s", taskDef)
}

// FindLogGroup looks up the log group from a service's task definition.
func FindLogGroup(ctx context.Context, client ECSClient, cluster, service string) (logGroup, streamPrefix string, err error) {
	type logGroupFinder interface {
		LogGroupForService(ctx context.Context, taskDef string) (string, string, error)
	}

	// Get the service to find its task definition
	svc, err := client.DescribeService(ctx, cluster, service)
	if err != nil {
		return "", "", err
	}

	// Unwrap cached client if needed
	var finder logGroupFinder
	switch c := client.(type) {
	case *CachedClient:
		finder = c.Client
	case *Client:
		finder = c
	default:
		return "", "", fmt.Errorf("unsupported client type for log group lookup")
	}

	return finder.LogGroupForService(ctx, svc.TaskDefinition)
}
