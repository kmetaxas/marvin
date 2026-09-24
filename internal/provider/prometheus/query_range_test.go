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

func TestQueryRangeTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &queryRangeTask{}
	assert.Equal(t, "prometheus.query.range", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestQueryRangeTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &queryRangeTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing query", map[string]any{"start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z"}, "missing required parameter: query"},
		{"missing start", map[string]any{"query": "up", "end": "2024-01-15T11:00:00Z"}, "missing required parameter: start"},
		{"missing end", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z"}, "missing required parameter: end"},
		{"invalid start", map[string]any{"query": "up", "start": "bad", "end": "2024-01-15T11:00:00Z"}, "start must be RFC3339"},
		{"invalid end", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "bad"}, "end must be RFC3339"},
		{"end before start", map[string]any{"query": "up", "start": "2024-01-15T11:00:00Z", "end": "2024-01-15T10:00:00Z"}, "end must be after start"},
		{"invalid step", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z", "step": "nope"}, "step must be a duration"},
		{"step too small", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z", "step": "1s"}, "step must be at least"},
		{"timeout zero", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z", "timeout_seconds": 0}, "timeout_seconds must be greater than 0"},
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

func TestQueryRangeTaskRangeExceedsGuardrail(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxQueryRange = 1 * time.Hour

	client := &fakePrometheusClient{guardrails: guardrails}
	task := &queryRangeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T12:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "exceeds maximum")
}

func TestQueryRangeTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &queryRangeTask{}
	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestQueryRangeTaskMatrixResult(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return model.Matrix{
				&model.SampleStream{
					Metric: model.Metric{"__name__": "up", "instance": "localhost:9090"},
					Values: []model.SamplePair{
						{Timestamp: model.Time(1720000000000), Value: 1},
						{Timestamp: model.Time(1720000015000), Value: 1},
					},
				},
			}, v1.Warnings{"warning-b"}, nil
		},
	}
	task := &queryRangeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
		"step":  "15s",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "matrix", data["result_type"])
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, []string{"warning-b"}, data["warnings"])

	result := data["result"].([]map[string]any)
	require.Len(t, result, 1)
	values := result[0]["values"].([]any)
	assert.Len(t, values, 2)
}

func TestQueryRangeTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxReturnedSeries = 1
	guardrails.MaxReturnedSamples = 1

	client := &fakePrometheusClient{
		guardrails: guardrails,
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return model.Matrix{
				&model.SampleStream{
					Metric: model.Metric{"__name__": "a"},
					Values: []model.SamplePair{
						{Timestamp: 1, Value: 1},
						{Timestamp: 2, Value: 2},
					},
				},
				&model.SampleStream{
					Metric: model.Metric{"__name__": "b"},
					Values: []model.SamplePair{{Timestamp: 1, Value: 1}},
				},
			}, nil, nil
		},
	}
	task := &queryRangeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	result := data["result"].([]map[string]any)
	assert.Len(t, result, 1)
}

func TestQueryRangeTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return nil, nil, errors.New("api failed")
		},
	}
	task := &queryRangeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestQueryRangeTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return nil, nil, context.Canceled
		},
	}
	task := &queryRangeTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
