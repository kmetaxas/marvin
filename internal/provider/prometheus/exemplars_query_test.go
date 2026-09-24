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

func TestExemplarsQueryTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &exemplarsQueryTask{}
	assert.Equal(t, "prometheus.exemplars.query", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestExemplarsQueryTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &exemplarsQueryTask{}
	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestExemplarsQueryTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &exemplarsQueryTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing query", map[string]any{"start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z"}, "missing required parameter: query"},
		{"missing start", map[string]any{"query": "up", "end": "2024-01-15T11:00:00Z"}, "missing required parameter: start"},
		{"missing end", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z"}, "missing required parameter: end"},
		{"bad start", map[string]any{"query": "up", "start": "not-a-date", "end": "2024-01-15T11:00:00Z"}, "start must be RFC3339"},
		{"bad end", map[string]any{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "not-a-date"}, "end must be RFC3339"},
		{"end before start", map[string]any{"query": "up", "start": "2024-01-15T11:00:00Z", "end": "2024-01-15T10:00:00Z"}, "end must be after start"},
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

func TestExemplarsQueryTaskRangeExceedsGuardrail(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxQueryRange = 1 * time.Hour

	client := &fakePrometheusClient{guardrails: guardrails}
	task := &exemplarsQueryTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T12:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "query range exceeds maximum allowed")
}

func TestExemplarsQueryTaskSuccess(t *testing.T) {
	t.Parallel()

	ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryExemplarsFunc: func(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
			return []v1.ExemplarQueryResult{
				{
					SeriesLabels: model.LabelSet{"__name__": "http_requests_total", "job": "api"},
					Exemplars: []v1.Exemplar{
						{
							Labels:    model.LabelSet{"trace_id": "abc123"},
							Value:     model.SampleValue(1.5),
							Timestamp: model.TimeFromUnix(ts.Unix()),
						},
					},
				},
			}, nil
		},
	}
	task := &exemplarsQueryTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "http_requests_total",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])

	exemplars := data["exemplars"].([]map[string]any)
	require.Len(t, exemplars, 1)

	seriesLabels := exemplars[0]["series_labels"].(map[string]string)
	assert.Equal(t, "http_requests_total", seriesLabels["__name__"])
	assert.Equal(t, "api", seriesLabels["job"])

	items := exemplars[0]["exemplars"].([]map[string]any)
	require.Len(t, items, 1)
	assert.Equal(t, "abc123", items[0]["labels"].(map[string]string)["trace_id"])
	assert.Equal(t, "1.5", items[0]["value"])
	assert.Equal(t, "2024-01-15T10:30:00Z", items[0]["timestamp"])
}

func TestExemplarsQueryTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		queryExemplarsFunc: func(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
			return nil, errors.New("exemplars failed")
		},
	}
	task := &exemplarsQueryTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{
		"query": "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T11:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "exemplars failed")
}
