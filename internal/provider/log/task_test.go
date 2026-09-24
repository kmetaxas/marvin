package log

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{
		configured: true,
		searchMessagesFunc: func(_ context.Context, req searchMessagesRequest) (*searchMessagesResponse, error) {
			assert.Equal(t, "source:web-01", req.Query)
			assert.Equal(t, []string{"timestamp", "source"}, req.Fields)
			assert.Equal(t, 50, req.Size)
			return &searchMessagesResponse{
				Messages:     []map[string]any{{"message": "hello"}},
				TotalResults: 1,
			}, nil
		},
	}
	p := newFakeClientProvider(client)
	search := &searchTask{provider: p}

	result, err := search.Execute(context.Background(), map[string]any{
		"query":   "source:web-01",
		"fields":  []string{"timestamp", "source"},
		"limit":   50,
		"streams": []string{"000000000000000000000001"},
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]any)
	assert.Equal(t, 1, data["total_results"])
	assert.Equal(t, 1, data["returned"])
	assert.False(t, data["truncated"].(bool))
	assert.NotNil(t, data["messages"])
}

func TestSearchTaskMetadataOnly(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{
		configured: true,
		searchMessagesFunc: func(_ context.Context, req searchMessagesRequest) (*searchMessagesResponse, error) {
			return &searchMessagesResponse{
				Messages:     []map[string]any{{"msg": "secret"}},
				TotalResults: 99,
			}, nil
		},
	}
	p := newFakeClientProvider(client)
	search := &searchTask{provider: p}

	result, err := search.Execute(context.Background(), map[string]any{
		"query":            "source:db",
		"include_messages": false,
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]any)
	assert.Equal(t, 99, data["total_results"])
	assert.Nil(t, data["messages"])
}

func TestSearchTaskNilClient(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.GraylogConfig{})
	search := &searchTask{provider: p}

	result, err := search.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "not configured")
}

func TestSearchTaskGuardrailRejection(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{
		configured: true,
		guardrails: logGuardrails{MaxQueryRange: 3600},
	}
	p := newFakeClientProvider(client)
	search := &searchTask{provider: p}

	result, err := search.Execute(context.Background(), map[string]any{
		"timerange": map[string]any{"type": "relative", "range": 7200},
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "exceeds maximum")
}

func TestCountTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{
		configured: true,
		searchAggregateFunc: func(_ context.Context, req searchAggregateRequest) (*searchAggregateResponse, error) {
			assert.Len(t, req.Metrics, 1)
			assert.Equal(t, "count", req.Metrics[0].Function)
			return &searchAggregateResponse{
				Rows: []aggregateRow{{Key: "", Values: map[string]any{"count": 42}}},
			}, nil
		},
	}
	p := newFakeClientProvider(client)
	count := &countTask{provider: p}

	result, err := count.Execute(context.Background(), map[string]any{
		"query": "level:3",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]any)
	assert.Equal(t, 42, data["count"])
}

func TestHistogramTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{
		configured: true,
		searchAggregateFunc: func(_ context.Context, req searchAggregateRequest) (*searchAggregateResponse, error) {
			assert.Len(t, req.GroupBy, 1)
			assert.Equal(t, "timestamp", req.GroupBy[0].Field)
			return &searchAggregateResponse{
				Rows: []aggregateRow{
					{Key: "2024-01-01T00:00:00Z", Values: map[string]any{"count": 10}},
					{Key: "2024-01-01T00:05:00Z", Values: map[string]any{"count": 20}},
				},
			}, nil
		},
	}
	p := newFakeClientProvider(client)
	hist := &histogramTask{provider: p}

	result, err := hist.Execute(context.Background(), map[string]any{
		"query":    "error",
		"interval": map[string]any{"value": 5, "unit": "m"},
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]any)
	assert.Equal(t, 30, data["total_count"])
	buckets := data["buckets"].([]map[string]any)
	assert.Len(t, buckets, 2)
}

func TestFieldstatsTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{
		configured: true,
		searchAggregateFunc: func(_ context.Context, req searchAggregateRequest) (*searchAggregateResponse, error) {
			assert.Len(t, req.GroupBy, 1)
			assert.Equal(t, "response_time_ms", req.GroupBy[0].Field)
			assert.Len(t, req.Metrics, 5)
			return &searchAggregateResponse{
				Rows: []aggregateRow{{
					Key:    "response_time_ms",
					Values: map[string]any{"count": 100, "min": 10, "max": 500, "avg": 120.5, "sum": 12050},
				}},
			}, nil
		},
	}
	p := newFakeClientProvider(client)
	fs := &fieldstatsTask{provider: p}

	result, err := fs.Execute(context.Background(), map[string]any{
		"field": "response_time_ms",
		"query": "source:api",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]any)
	assert.Equal(t, "response_time_ms", data["field"])
	stats := data["stats"].(map[string]any)
	assert.Equal(t, 100, stats["count"])
	assert.Equal(t, 10, stats["min"])
	assert.Equal(t, 500, stats["max"])
}

func TestFieldstatsTaskMissingField(t *testing.T) {
	t.Parallel()

	client := &fakeGraylogClient{configured: true}
	p := newFakeClientProvider(client)
	fs := &fieldstatsTask{provider: p}

	result, err := fs.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "field")
}
