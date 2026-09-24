package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetMetadataTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &targetMetadataTask{}
	assert.Equal(t, "prometheus.target.metadata", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTargetMetadataTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &targetMetadataTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"match_target wrong type", map[string]any{"match_target": "x"}, "match_target must be a map of strings"},
		{"match_target non-string value", map[string]any{"match_target": map[string]any{"a": 1}}, "match_target must be a map of strings"},
		{"metric wrong type", map[string]any{"metric": 1}, "metric must be a string"},
		{"limit zero", map[string]any{"limit": 0}, "limit must be greater than 0"},
		{"limit wrong type", map[string]any{"limit": "10"}, "limit must be an integer"},
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

func TestTargetMetadataTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &targetMetadataTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTargetMetadataTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsMetadataFunc: func(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
			assert.Equal(t, "", matchTarget)
			assert.Equal(t, "", metric)
			return []v1.MetricMetadata{
				{
					Target: map[string]string{"job": "prometheus", "instance": "localhost:9090"},
					Metric: "up",
					Type:   v1.MetricTypeGauge,
					Help:   "up metric",
					Unit:   "",
				},
			}, nil
		},
	}
	task := &targetMetadataTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 1, data["count"])

	metadata := data["metadata"].([]map[string]any)
	require.Len(t, metadata, 1)
	assert.Equal(t, "up", metadata[0]["metric"])
	assert.Equal(t, "gauge", metadata[0]["type"])
	assert.Equal(t, "up metric", metadata[0]["help"])
	assert.Equal(t, "prometheus", metadata[0]["target"].(map[string]string)["job"])
}

func TestTargetMetadataTaskMatchTargetAndMetric(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsMetadataFunc: func(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
			assert.Equal(t, `{job="prometheus"}`, matchTarget)
			assert.Equal(t, "up", metric)
			return []v1.MetricMetadata{
				{Target: map[string]string{"job": "prometheus"}, Metric: "up", Type: v1.MetricTypeGauge},
			}, nil
		},
	}
	task := &targetMetadataTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"match_target": map[string]string{"job": "prometheus"},
		"metric":       "up",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
}

func TestTargetMetadataTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		targetsMetadataFunc: func(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
			return []v1.MetricMetadata{
				{Metric: "a", Type: v1.MetricTypeGauge},
				{Metric: "b", Type: v1.MetricTypeGauge},
				{Metric: "c", Type: v1.MetricTypeGauge},
			}, nil
		},
	}
	task := &targetMetadataTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	metadata := data["metadata"].([]map[string]any)
	assert.Len(t, metadata, 2)
}

func TestTargetMetadataTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsMetadataFunc: func(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
			return nil, errors.New("api failed")
		},
	}
	task := &targetMetadataTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestTargetMetadataTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		targetsMetadataFunc: func(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
			return nil, context.Canceled
		},
	}
	task := &targetMetadataTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
