package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusTsdbTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &statusTsdbTask{}
	assert.Equal(t, "prometheus.status.tsdb", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStatusTsdbTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &statusTsdbTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStatusTsdbTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &statusTsdbTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"limit": -1})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be greater than or equal to 0")

	res, err = task.Execute(context.Background(), map[string]any{"limit": "bogus"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit must be an integer")
}

func TestStatusTsdbTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		tsdbFunc: func(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
			return v1.TSDBResult{
				HeadStats: v1.TSDBHeadStats{
					NumSeries:     100,
					NumLabelPairs: 20,
					ChunkCount:    500,
					MinTime:       1000,
					MaxTime:       2000,
				},
				SeriesCountByMetricName: []v1.Stat{
					{Name: "up", Value: 10},
					{Name: "http_requests_total", Value: 5},
				},
				LabelValueCountByLabelName: []v1.Stat{
					{Name: "job", Value: 3},
				},
				MemoryInBytesByLabelName: []v1.Stat{
					{Name: "instance", Value: 1024},
				},
				SeriesCountByLabelValuePair: []v1.Stat{
					{Name: "job=prometheus", Value: 7},
				},
			}, nil
		},
	}
	task := &statusTsdbTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	head := data["head_stats"].(map[string]any)
	assert.Equal(t, 100, head["num_series"])
	assert.Equal(t, 20, head["num_label_pairs"])
	assert.Equal(t, 500, head["chunk_count"])
	assert.Equal(t, 1000, head["min_time"])
	assert.Equal(t, 2000, head["max_time"])

	seriesByMetric := data["series_count_by_metric_name"].([]map[string]any)
	require.Len(t, seriesByMetric, 2)
	assert.Equal(t, "up", seriesByMetric[0]["name"])
	assert.Equal(t, uint64(10), seriesByMetric[0]["value"])

	assert.Equal(t, false, data["truncated"])
}

func TestStatusTsdbTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		tsdbFunc: func(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
			return v1.TSDBResult{
				SeriesCountByMetricName: []v1.Stat{
					{Name: "a", Value: 1},
					{Name: "b", Value: 2},
					{Name: "c", Value: 3},
					{Name: "d", Value: 4},
				},
			}, nil
		},
	}
	task := &statusTsdbTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 2})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	seriesByMetric := data["series_count_by_metric_name"].([]map[string]any)
	require.Len(t, seriesByMetric, 2)
	assert.Equal(t, true, data["truncated"])
}

func TestStatusTsdbTaskLimitCappedByGuardrail(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxMetadataResults = 1

	client := &fakePrometheusClient{
		guardrails: guardrails,
		tsdbFunc: func(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
			return v1.TSDBResult{
				SeriesCountByMetricName: []v1.Stat{
					{Name: "a", Value: 1},
					{Name: "b", Value: 2},
				},
			}, nil
		},
	}
	task := &statusTsdbTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 100})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	seriesByMetric := data["series_count_by_metric_name"].([]map[string]any)
	require.Len(t, seriesByMetric, 1)
	assert.Equal(t, true, data["truncated"])
}

func TestStatusTsdbTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		tsdbFunc: func(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
			return v1.TSDBResult{}, errors.New("tsdb failed")
		},
	}
	task := &statusTsdbTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "tsdb failed")
}
