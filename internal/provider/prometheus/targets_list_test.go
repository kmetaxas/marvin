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

func TestTargetsListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &targetsListTask{}
	assert.Equal(t, "prometheus.targets.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTargetsListTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"state invalid", map[string]any{"state": "bogus"}, "state must be one of"},
		{"state wrong type", map[string]any{"state": 1}, "state must be a string"},
		{"job wrong type", map[string]any{"job": 1}, "job must be a string"},
		{"instance wrong type", map[string]any{"instance": 1}, "instance must be a string"},
		{"limit zero", map[string]any{"limit": 0}, "limit must be greater than 0"},
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

func TestTargetsListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &targetsListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTargetsListTaskSuccess(t *testing.T) {
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
	task := &targetsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 1, data["count"])

	targets := data["targets"].([]map[string]any)
	require.Len(t, targets, 1)
	assert.Equal(t, "http://localhost:9090/metrics", targets[0]["scrape_url"])
	assert.Equal(t, "prometheus", targets[0]["job"])
	assert.Equal(t, "localhost:9090", targets[0]["instance"])
	assert.Equal(t, "up", targets[0]["health"])
	assert.Equal(t, "2024-01-15T10:30:00Z", targets[0]["last_scrape"])
	assert.Equal(t, float64(50), targets[0]["scrape_duration_ms"])
	assert.Equal(t, "prometheus", targets[0]["scrape_pool"])
}

func TestTargetsListTaskDroppedState(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{Labels: model.LabelSet{"job": "a", "instance": "i1"}, Health: v1.HealthGood},
				},
				Dropped: []v1.DroppedTarget{
					{DiscoveredLabels: map[string]string{"job": "b", "instance": "i2"}},
				},
			}, nil
		},
	}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"state": "dropped"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	targets := data["targets"].([]map[string]any)
	require.Len(t, targets, 1)
	assert.Equal(t, "dropped", targets[0]["health"])
	assert.Equal(t, "b", targets[0]["job"])
}

func TestTargetsListTaskAnyState(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{Labels: model.LabelSet{"job": "a", "instance": "i1"}, Health: v1.HealthGood},
				},
				Dropped: []v1.DroppedTarget{
					{DiscoveredLabels: map[string]string{"job": "b", "instance": "i2"}},
				},
			}, nil
		},
	}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"state": "any"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func TestTargetsListTaskJobInstanceFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{Labels: model.LabelSet{"job": "a", "instance": "i1"}, Health: v1.HealthGood},
					{Labels: model.LabelSet{"job": "a", "instance": "i2"}, Health: v1.HealthGood},
					{Labels: model.LabelSet{"job": "b", "instance": "i1"}, Health: v1.HealthGood},
				},
			}, nil
		},
	}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"job": "a", "instance": "i1"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	targets := data["targets"].([]map[string]any)
	require.Len(t, targets, 1)
	assert.Equal(t, "a", targets[0]["job"])
	assert.Equal(t, "i1", targets[0]["instance"])
}

func TestTargetsListTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxReturnedSeries = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{
				Active: []v1.ActiveTarget{
					{Labels: model.LabelSet{"job": "a", "instance": "i1"}},
					{Labels: model.LabelSet{"job": "a", "instance": "i2"}},
					{Labels: model.LabelSet{"job": "a", "instance": "i3"}},
				},
			}, nil
		},
	}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	targets := data["targets"].([]map[string]any)
	assert.Len(t, targets, 2)
}

func TestTargetsListTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{}, errors.New("api failed")
		},
	}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestTargetsListTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsFunc: func(ctx context.Context) (v1.TargetsResult, error) {
			return v1.TargetsResult{}, context.Canceled
		},
	}
	task := &targetsListTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
