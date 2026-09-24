package prometheus

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &targetGetTask{}
	assert.Equal(t, "prometheus.target.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTargetGetTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &targetGetTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"no filters", map[string]any{}, "at least one of job, instance, scrape_pool"},
		{"job wrong type", map[string]any{"job": 1}, "job must be a string"},
		{"instance wrong type", map[string]any{"instance": 1}, "instance must be a string"},
		{"scrape_pool wrong type", map[string]any{"scrape_pool": 1}, "scrape_pool must be a string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := task.Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.False(t, res.Success)
			assert.Contains(t, res.Error, tt.want)
		})
	}
}

func TestTargetGetTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &targetGetTask{}
	res, err := task.Execute(context.Background(), map[string]any{"job": "a"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTargetGetTaskSuccess(t *testing.T) {
	t.Parallel()

	lastScrape := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{
						ScrapeURL:          "http://localhost:9090/metrics",
						ScrapePool:         "prometheus",
						Labels:             model.LabelSet{"job": "prometheus", "instance": "localhost:9090"},
						DiscoveredLabels:   map[string]string{"__address__": "localhost:9090"},
						Health:             v1.HealthGood,
						LastScrape:         lastScrape,
						LastScrapeDuration: 0.05,
					},
				},
			}, nil
		},
	}
	task := &targetGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"job": "prometheus"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "http://localhost:9090/metrics", data["scrape_url"])
	assert.Equal(t, "prometheus", data["job"])
	assert.Equal(t, "localhost:9090", data["instance"])
	assert.Equal(t, "up", data["health"])
	assert.Equal(t, "2024-01-15T10:30:00Z", data["last_scrape"])
	assert.Equal(t, float64(50), data["scrape_duration_ms"])
}

func TestTargetGetTaskScrapePoolFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{ScrapePool: "pool-a", Labels: model.LabelSet{"job": "a", "instance": "i1"}},
					{ScrapePool: "pool-b", Labels: model.LabelSet{"job": "b", "instance": "i2"}},
				},
			}, nil
		},
	}
	task := &targetGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"scrape_pool": "pool-b"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "b", data["job"])
	assert.Equal(t, "pool-b", data["scrape_pool"])
}

func TestTargetGetTaskNotFound(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{Labels: model.LabelSet{"job": "a", "instance": "i1"}},
				},
			}, nil
		},
	}
	task := &targetGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"job": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "target not found")
}

func TestTargetGetTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{}, errors.New("api failed")
		},
	}
	task := &targetGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"job": "a"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestTargetGetTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{}, context.Canceled
		},
	}
	task := &targetGetTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{"job": "a"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
