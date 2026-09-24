package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &metadataGetTask{}
	assert.Equal(t, "prometheus.metadata.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestMetadataGetTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &metadataGetTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"metric wrong type", map[string]any{"metric": 123}, "metric must be a string"},
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

func TestMetadataGetTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &metadataGetTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestMetadataGetTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
			assert.Equal(t, "", metric)
			return map[string][]v1.Metadata{
				"up": {
					{Type: v1.MetricTypeCounter, Help: "up metric", Unit: ""},
				},
				"http_requests_total": {
					{Type: v1.MetricTypeCounter, Help: "total requests", Unit: ""},
				},
			}, nil
		},
	}
	task := &metadataGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 2, data["count"])

	metadata := data["metadata"].([]map[string]any)
	require.Len(t, metadata, 2)
	assert.Equal(t, "up", metadata[0]["metric"])
	assert.Equal(t, "counter", metadata[0]["type"])
	assert.Equal(t, "up metric", metadata[0]["help"])
	assert.Equal(t, "", metadata[0]["unit"])
}

func TestMetadataGetTaskMetricFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
			assert.Equal(t, "up", metric)
			return map[string][]v1.Metadata{
				"up": {
					{Type: v1.MetricTypeGauge, Help: "up metric", Unit: ""},
				},
			}, nil
		},
	}
	task := &metadataGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"metric": "up"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	metadata := data["metadata"].([]map[string]any)
	require.Len(t, metadata, 1)
	assert.Equal(t, "gauge", metadata[0]["type"])
}

func TestMetadataGetTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		metadataFunc: func(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
			return map[string][]v1.Metadata{
				"a": {{Type: v1.MetricTypeCounter}},
				"b": {{Type: v1.MetricTypeCounter}},
				"c": {{Type: v1.MetricTypeCounter}},
				"d": {{Type: v1.MetricTypeCounter}},
			}, nil
		},
	}
	task := &metadataGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	metadata := data["metadata"].([]map[string]any)
	assert.Len(t, metadata, 2)
}

func TestMetadataGetTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
			return nil, errors.New("api failed")
		},
	}
	task := &metadataGetTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestMetadataGetTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
			return nil, context.Canceled
		},
	}
	task := &metadataGetTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
