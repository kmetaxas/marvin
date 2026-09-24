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

func TestQueryInstantTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &queryInstantTask{}
	assert.Equal(t, "prometheus.query.instant", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestQueryInstantTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing query", map[string]any{}, "missing required parameter: query"},
		{"empty query", map[string]any{"query": "  "}, "missing required parameter: query"},
		{"query wrong type", map[string]any{"query": 123}, "missing required parameter: query"},
		{"invalid time", map[string]any{"query": "up", "time": "not-a-date"}, "must be RFC3339"},
		{"timeout zero", map[string]any{"query": "up", "timeout_seconds": 0}, "timeout_seconds must be greater than 0"},
		{"timeout wrong type", map[string]any{"query": "up", "timeout_seconds": "30"}, "timeout_seconds must be an integer"},
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

func TestQueryInstantTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &queryInstantTask{}
	res, err := task.Execute(context.Background(), map[string]any{"query": "up"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestQueryInstantTaskVectorResult(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return model.Vector{
				&model.Sample{
					Metric:    model.Metric{"__name__": "up", "instance": "localhost:9090"},
					Value:     model.SampleValue(1),
					Timestamp: model.Time(1720000000000),
				},
			}, v1.Warnings{"warning-a"}, nil
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"query": "up"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "vector", data["result_type"])
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, []string{"warning-a"}, data["warnings"])

	result := data["result"].([]map[string]any)
	require.Len(t, result, 1)
	assert.Equal(t, "up", result[0]["metric"].(map[string]string)["__name__"])
}

func TestQueryInstantTaskScalarResult(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return &model.Scalar{Value: model.SampleValue(42), Timestamp: model.Time(1720000000000)}, nil, nil
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"query": "42"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "scalar", data["result_type"])
	result := data["result"].([]any)
	require.Len(t, result, 2)
	assert.Equal(t, "42", result[1])
}

func TestQueryInstantTaskStringResult(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return &model.String{Value: "hello", Timestamp: model.Time(1720000000000)}, nil, nil
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"query": `"hello"`})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "string", data["result_type"])
	result := data["result"].([]any)
	assert.Equal(t, "hello", result[1])
}

func TestQueryInstantTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxReturnedSeries = 2
	guardrails.MaxReturnedSamples = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return model.Vector{
				&model.Sample{Metric: model.Metric{"__name__": "a"}, Value: 1, Timestamp: 1},
				&model.Sample{Metric: model.Metric{"__name__": "b"}, Value: 2, Timestamp: 1},
				&model.Sample{Metric: model.Metric{"__name__": "c"}, Value: 3, Timestamp: 1},
			}, nil, nil
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"query": "up"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	result := data["result"].([]map[string]any)
	assert.Len(t, result, 2)
}

func TestQueryInstantTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return nil, nil, errors.New("api failed")
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"query": "up"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestQueryInstantTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			return nil, nil, context.Canceled
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{"query": "up"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}

func TestQueryInstantTaskTimeoutClamped(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.QueryTimeout = 30 * time.Second

	var capturedOpts []v1.Option
	client := &fakePrometheusClient{
		guardrails: guardrails,
		queryFunc: func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
			capturedOpts = opts
			return model.Vector{}, nil, nil
		},
	}
	task := &queryInstantTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"query": "up", "timeout_seconds": 300})
	require.NoError(t, err)
	assert.True(t, res.Success)
	require.NotEmpty(t, capturedOpts)
}
