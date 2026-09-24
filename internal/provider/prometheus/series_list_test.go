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

func TestSeriesListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &seriesListTask{}
	assert.Equal(t, "prometheus.series.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestSeriesListTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &seriesListTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing match", map[string]any{}, "missing required parameter: match"},
		{"empty match", map[string]any{"match": []string{}}, "missing required parameter: match"},
		{"match wrong type", map[string]any{"match": "up"}, "must be an array of strings"},
		{"match non-string items", map[string]any{"match": []any{1, 2}}, "must be an array of strings"},
		{"invalid start", map[string]any{"match": []string{"up"}, "start": "bad"}, "start must be RFC3339"},
		{"invalid end", map[string]any{"match": []string{"up"}, "end": "bad"}, "end must be RFC3339"},
		{"limit zero", map[string]any{"match": []string{"up"}, "limit": 0}, "limit must be greater than 0"},
		{"limit wrong type", map[string]any{"match": []string{"up"}, "limit": "10"}, "limit must be an integer"},
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

func TestSeriesListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &seriesListTask{}
	res, err := task.Execute(context.Background(), map[string]any{"match": []string{"up"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestSeriesListTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		seriesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
			return []model.LabelSet{
				{"__name__": "up", "instance": "localhost:9090", "job": "prometheus"},
			}, nil, nil
		},
	}
	task := &seriesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"match": []string{"up"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 1, data["count"])

	series := data["series"].([]map[string]string)
	require.Len(t, series, 1)
	assert.Equal(t, "up", series[0]["__name__"])
}

func TestSeriesListTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxReturnedSeries = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		seriesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
			return []model.LabelSet{
				{"__name__": "a"},
				{"__name__": "b"},
				{"__name__": "c"},
			}, nil, nil
		},
	}
	task := &seriesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"match": []string{"up"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	series := data["series"].([]map[string]string)
	assert.Len(t, series, 2)
}

func TestSeriesListTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		seriesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
			return nil, nil, errors.New("api failed")
		},
	}
	task := &seriesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"match": []string{"up"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestSeriesListTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		seriesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
			return nil, nil, context.Canceled
		},
	}
	task := &seriesListTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{"match": []string{"up"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
