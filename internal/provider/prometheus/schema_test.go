package prometheus

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestQueryInstantJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(queryInstantSchema)))
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestQueryRangeJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(queryRangeSchema)))
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestSeriesListJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(seriesListSchema)))
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestMetricNamesJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(metricNamesSchema)))
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestQueryInstantSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(queryInstantSchema)))
	require.NoError(t, err)

	valid := []map[string]any{
		{"query": "up"},
		{"query": "up", "time": "2024-01-15T10:30:00Z"},
		{"query": "up", "timeout_seconds": 60},
	}
	for _, params := range valid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{},
		{"query": 123},
		{"query": "up", "timeout_seconds": 0},
		{"query": "up", "timeout_seconds": 1000},
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}

func TestQueryRangeSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(queryRangeSchema)))
	require.NoError(t, err)

	valid := []map[string]any{
		{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z"},
		{"query": "up", "start": "2024-01-15T10:00:00Z", "end": "2024-01-15T11:00:00Z", "step": "15s"},
	}
	for _, params := range valid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{"query": "up"},
		{"query": "up", "start": "2024-01-15T10:00:00Z"},
		{"query": "up", "start": "not-a-date", "end": "2024-01-15T11:00:00Z"},
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}

func TestSeriesListSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(seriesListSchema)))
	require.NoError(t, err)

	valid := []map[string]any{
		{"match": []string{"up"}},
		{"match": []string{"up", `job="prometheus"`}, "limit": 10},
	}
	for _, params := range valid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{},
		{"match": []string{}},
		{"match": "up"},
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}

func TestMetricNamesSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(metricNamesSchema)))
	require.NoError(t, err)

	valid := []map[string]any{
		{},
		{"search": "http"},
		{"match": []string{"job=\"prometheus\""}, "limit": 50},
	}
	for _, params := range valid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{"search": 123},
		{"limit": 0},
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}
