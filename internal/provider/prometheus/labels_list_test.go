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

func TestLabelsListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &labelsListTask{}
	assert.Equal(t, "prometheus.labels.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLabelsListTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &labelsListTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"match wrong type", map[string]any{"match": "up"}, "must be an array of strings"},
		{"match non-string items", map[string]any{"match": []any{1}}, "must be an array of strings"},
		{"invalid start", map[string]any{"start": "bad"}, "start must be RFC3339"},
		{"invalid end", map[string]any{"end": "bad"}, "end must be RFC3339"},
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

func TestLabelsListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &labelsListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLabelsListTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelNamesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error) {
			return model.LabelNames{"__name__", "job", "instance"}, nil, nil
		},
	}
	task := &labelsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 3, data["count"])
	labels := data["labels"].([]string)
	assert.Len(t, labels, 3)
	assert.Equal(t, "__name__", labels[0])
}

func TestLabelsListTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		labelNamesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error) {
			return model.LabelNames{"a", "b", "c", "d"}, nil, nil
		},
	}
	task := &labelsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	labels := data["labels"].([]string)
	assert.Len(t, labels, 2)
}

func TestLabelsListTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelNamesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error) {
			return nil, nil, errors.New("api failed")
		},
	}
	task := &labelsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestLabelsListTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		labelNamesFunc: func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error) {
			return nil, nil, context.Canceled
		},
	}
	task := &labelsListTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
