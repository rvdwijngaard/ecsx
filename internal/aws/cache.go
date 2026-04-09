package aws

import (
	"context"
	"time"

	"github.com/samber/hot"
)

const cacheTTL = 5 * time.Minute
const cacheCapacity = 1000

func newCache[T any](ttl ...time.Duration) *hot.HotCache[string, T] {
	t := cacheTTL
	if len(ttl) > 0 {
		t = ttl[0]
	}
	return hot.NewHotCache[string, T](hot.LRU, cacheCapacity).WithTTL(t).Build()
}

func cachedGet[T any](cache *hot.HotCache[string, T], key string, fetch func() (T, error)) (T, error) {
	if val, found, _ := cache.Get(key); found {
		return val, nil
	}
	result, err := fetch()
	if err != nil {
		var zero T
		return zero, err
	}
	cache.Set(key, result)
	return result, nil
}

type CachedClient struct {
	*Client
	clusters           *hot.HotCache[string, []Cluster]
	svcNames           *hot.HotCache[string, []string]
	svcDetail          *hot.HotCache[string, *Service]
	services           *hot.HotCache[string, []Service]
	tasks              *hot.HotCache[string, []Task]
	taskDefs           *hot.HotCache[string, []ContainerDefinition]
	metrics            *hot.HotCache[string, *ServiceMetrics]
	containerInstances *hot.HotCache[string, []ContainerInstance]
}

func NewCachedClient(profile, region string) (*CachedClient, error) {
	client, err := NewClient(profile, region)
	if err != nil {
		return nil, err
	}
	return &CachedClient{
		Client:             client,
		clusters:           newCache[[]Cluster](),
		svcNames:           newCache[[]string](),
		svcDetail:          newCache[*Service](),
		services:           newCache[[]Service](),
		tasks:              newCache[[]Task](),
		taskDefs:           newCache[[]ContainerDefinition](),
		metrics:            newCache[*ServiceMetrics](2 * time.Minute),
		containerInstances: newCache[[]ContainerInstance](),
	}, nil
}

func (c *CachedClient) ListClusters(ctx context.Context) ([]Cluster, error) {
	return cachedGet(c.clusters, "all", func() ([]Cluster, error) {
		return c.Client.ListClusters(ctx)
	})
}

func (c *CachedClient) ListContainerInstances(ctx context.Context, cluster string) ([]ContainerInstance, error) {
	return cachedGet(c.containerInstances, cluster, func() ([]ContainerInstance, error) {
		return c.Client.ListContainerInstances(ctx, cluster)
	})
}

func (c *CachedClient) ListServiceNames(ctx context.Context, cluster string) ([]string, error) {
	return cachedGet(c.svcNames, cluster, func() ([]string, error) {
		return c.Client.ListServiceNames(ctx, cluster)
	})
}

func (c *CachedClient) DescribeService(ctx context.Context, cluster, service string) (*Service, error) {
	return cachedGet(c.svcDetail, cluster+"/"+service, func() (*Service, error) {
		return c.Client.DescribeService(ctx, cluster, service)
	})
}

func (c *CachedClient) ListServices(ctx context.Context, cluster string) ([]Service, error) {
	return cachedGet(c.services, cluster, func() ([]Service, error) {
		return c.Client.ListServices(ctx, cluster)
	})
}

func (c *CachedClient) ListTasks(ctx context.Context, cluster, service string) ([]Task, error) {
	return cachedGet(c.tasks, cluster+"/"+service, func() ([]Task, error) {
		return c.Client.ListTasks(ctx, cluster, service)
	})
}

func (c *CachedClient) DescribeTaskDefinition(ctx context.Context, taskDef string) ([]ContainerDefinition, error) {
	return cachedGet(c.taskDefs, taskDef, func() ([]ContainerDefinition, error) {
		return c.Client.DescribeTaskDefinition(ctx, taskDef)
	})
}

func (c *CachedClient) GetServiceMetrics(ctx context.Context, cluster, service string) (*ServiceMetrics, error) {
	return cachedGet(c.metrics, cluster+"/"+service, func() (*ServiceMetrics, error) {
		return c.Client.GetServiceMetrics(ctx, cluster, service)
	})
}

func (c *CachedClient) UpdateServiceDesiredCount(ctx context.Context, cluster, service string, desiredCount int32) error {
	err := c.Client.UpdateServiceDesiredCount(ctx, cluster, service, desiredCount)
	if err != nil {
		return err
	}
	c.services.Delete(cluster)
	c.svcDetail.Delete(cluster + "/" + service)
	c.tasks.Delete(cluster + "/" + service)
	return nil
}

func (c *CachedClient) Purge() {
	c.clusters.Purge()
	c.svcNames.Purge()
	c.svcDetail.Purge()
	c.services.Purge()
	c.tasks.Purge()
	c.taskDefs.Purge()
	c.metrics.Purge()
	c.containerInstances.Purge()
}

func (c *CachedClient) TailLogs(ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string) (<-chan LogEvent, error) {
	return c.Client.TailLogs(ctx, logGroup, logStreamPrefix, filterPattern)
}

func (c *CachedClient) FetchRecentLogs(ctx context.Context, logGroup string, logStreamPrefix string, filterPattern string, start time.Time, end *time.Time) ([]LogEvent, error) {
	return c.Client.FetchRecentLogs(ctx, logGroup, logStreamPrefix, filterPattern, start, end)
}
