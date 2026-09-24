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

func TestMetricNamesTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &metricNamesTask{}
	assert.Equal(t, "prometheus.metric.names", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestMetricNamesTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &metricNamesTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"match wrong type", map[string]any{"match": "up"}, "must be an array of strings"},
		{"match non-string items", map[string]any{"match": []any{1}}, "must be an array of strings"},
		{"search wrong type", map[string]any{"search": 123}, "search must be a string"},
		{"invalid start", map[string]any{"start": "bad"}, "start must be RFC3339"},
		{"invalid end", map[string]any{"end": "bad"}, "end must be RFC3339"},
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

func TestMetricNamesTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &metricNamesTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestMetricNamesTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			assert.Equal(t, "__name__", label)
			return model.LabelValues{"up", "http_requests_total", "node_cpu_seconds_total"}, nil, nil
		},
	}
	task := &metricNamesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 3, data["count"])
	names := data["names"].([]string)
	assert.Len(t, names, 3)
}

func TestMetricNamesTaskSearchFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return model.LabelValues{"up", "http_requests_total", "node_cpu_seconds_total"}, nil, nil
		},
	}
	task := &metricNamesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"search": "http"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	names := data["names"].([]string)
	require.Len(t, names, 1)
	assert.Equal(t, "http_requests_total", names[0])
}

func TestMetricNamesTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return model.LabelValues{"a", "b", "c", "d"}, nil, nil
		},
	}
	task := &metricNamesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	names := data["names"].([]string)
	assert.Len(t, names, 2)
}

func TestMetricNamesTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return nil, nil, errors.New("api failed")
		},
	}
	task := &metricNamesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestMetricNamesTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return nil, nil, context.Canceled
		},
	}
	task := &metricNamesTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
