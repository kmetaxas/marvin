package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrapePoolsListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &scrapePoolsListTask{}
	assert.Equal(t, "prometheus.scrape_pools.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestScrapePoolsListTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &scrapePoolsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"include_counts": "yes"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "include_counts must be a boolean")
}

func TestScrapePoolsListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &scrapePoolsListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestScrapePoolsListTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{ScrapePool: "pool-a", Health: v1.HealthGood},
					{ScrapePool: "pool-a", Health: v1.HealthGood},
					{ScrapePool: "pool-a", Health: v1.HealthBad},
					{ScrapePool: "pool-b", Health: v1.HealthGood},
				},
				Dropped: []v1.DroppedTarget{
					{DiscoveredLabels: map[string]string{"scrape_pool": "pool-a"}},
				},
			}, nil
		},
	}
	task := &scrapePoolsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])

	pools := data["pools"].([]map[string]any)
	require.Len(t, pools, 2)

	byName := map[string]map[string]any{}
	for _, p := range pools {
		byName[p["name"].(string)] = p
	}

	poolA := byName["pool-a"]
	require.NotNil(t, poolA)
	assert.Equal(t, 4, poolA["total"])
	assert.Equal(t, 2, poolA["healthy"])
	assert.Equal(t, 1, poolA["unhealthy"])
	assert.Equal(t, 1, poolA["dropped"])

	poolB := byName["pool-b"]
	require.NotNil(t, poolB)
	assert.Equal(t, 1, poolB["total"])
	assert.Equal(t, 1, poolB["healthy"])
	assert.Equal(t, 0, poolB["unhealthy"])
	assert.Equal(t, 0, poolB["dropped"])
}

func TestScrapePoolsListTaskNoCounts(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{ScrapePool: "pool-a", Health: v1.HealthGood},
				},
			}, nil
		},
	}
	task := &scrapePoolsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"include_counts": false})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])

	pools := data["pools"].([]map[string]any)
	require.Len(t, pools, 1)
	assert.Equal(t, "pool-a", pools[0]["name"])
	_, hasTotal := pools[0]["total"]
	assert.False(t, hasTotal)
}

func TestScrapePoolsListTaskDroppedFallbackToJob(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Dropped: []v1.DroppedTarget{
					{DiscoveredLabels: map[string]string{"job": "myjob"}},
				},
			}, nil
		},
	}
	task := &scrapePoolsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	pools := data["pools"].([]map[string]any)
	require.Len(t, pools, 1)
	assert.Equal(t, "myjob", pools[0]["name"])
	assert.Equal(t, 1, pools[0]["dropped"])
}

func TestScrapePoolsListTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{}, errors.New("api failed")
		},
	}
	task := &scrapePoolsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestScrapePoolsListTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{}, context.Canceled
		},
	}
	task := &scrapePoolsListTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
