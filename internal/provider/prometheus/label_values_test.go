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

func TestLabelValuesTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &labelValuesTask{}
	assert.Equal(t, "prometheus.label.values", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLabelValuesTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &labelValuesTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing label", map[string]any{}, "missing required parameter: label"},
		{"empty label", map[string]any{"label": "  "}, "missing required parameter: label"},
		{"label wrong type", map[string]any{"label": 123}, "missing required parameter: label"},
		{"match wrong type", map[string]any{"label": "job", "match": "up"}, "must be an array of strings"},
		{"match non-string items", map[string]any{"label": "job", "match": []any{1}}, "must be an array of strings"},
		{"invalid start", map[string]any{"label": "job", "start": "bad"}, "start must be RFC3339"},
		{"invalid end", map[string]any{"label": "job", "end": "bad"}, "end must be RFC3339"},
		{"limit zero", map[string]any{"label": "job", "limit": 0}, "limit must be greater than 0"},
		{"limit wrong type", map[string]any{"label": "job", "limit": "10"}, "limit must be an integer"},
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

func TestLabelValuesTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &labelValuesTask{}
	res, err := task.Execute(context.Background(), map[string]any{"label": "job"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLabelValuesTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			assert.Equal(t, "job", label)
			return model.LabelValues{"prometheus", "node", "kubelet"}, nil, nil
		},
	}
	task := &labelValuesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"label": "job"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 3, data["count"])
	values := data["values"].([]string)
	assert.Len(t, values, 3)
	assert.Equal(t, "prometheus", values[0])
}

func TestLabelValuesTaskLimitClamp(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return model.LabelValues{"a", "b", "c", "d"}, nil, nil
		},
	}
	task := &labelValuesTask{provider: fakeClientProvider(client)}

	// limit above guardrail is clamped, then result truncated.
	res, err := task.Execute(context.Background(), map[string]any{"label": "job", "limit": 100})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	values := data["values"].([]string)
	assert.Len(t, values, 2)
}

func TestLabelValuesTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return model.LabelValues{"a", "b", "c", "d"}, nil, nil
		},
	}
	task := &labelValuesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"label": "job"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	values := data["values"].([]string)
	assert.Len(t, values, 2)
}

func TestLabelValuesTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return nil, nil, errors.New("api failed")
		},
	}
	task := &labelValuesTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"label": "job"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestLabelValuesTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelValuesFunc: func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
			return nil, nil, context.Canceled
		},
	}
	task := &labelValuesTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{"label": "job"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
