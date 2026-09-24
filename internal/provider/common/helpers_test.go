package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireString(t *testing.T) {
	t.Parallel()

	s, err := RequireString(map[string]any{"key": "value"}, "key")
	require.NoError(t, err)
	assert.Equal(t, "value", s)

	s, err = RequireString(map[string]any{"key": "  value  "}, "key")
	require.NoError(t, err)
	assert.Equal(t, "value", s)

	_, err = RequireString(map[string]any{}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required parameter")

	_, err = RequireString(map[string]any{"key": ""}, "key")
	require.Error(t, err)

	_, err = RequireString(map[string]any{"key": 123}, "key")
	require.Error(t, err)
}

func TestOptionalString(t *testing.T) {
	t.Parallel()

	s, err := OptionalString(map[string]any{"key": "value"}, "key", "default")
	require.NoError(t, err)
	assert.Equal(t, "value", s)

	s, err = OptionalString(map[string]any{"key": "  "}, "key", "default")
	require.NoError(t, err)
	assert.Equal(t, "default", s)

	s, err = OptionalString(map[string]any{}, "key", "default")
	require.NoError(t, err)
	assert.Equal(t, "default", s)

	_, err = OptionalString(map[string]any{"key": 123}, "key", "default")
	require.Error(t, err)
}

func TestOptionalBool(t *testing.T) {
	t.Parallel()

	b, err := OptionalBool(map[string]any{"key": true}, "key", false)
	require.NoError(t, err)
	assert.True(t, b)

	b, err = OptionalBool(map[string]any{}, "key", false)
	require.NoError(t, err)
	assert.False(t, b)

	_, err = OptionalBool(map[string]any{"key": "yes"}, "key", false)
	require.Error(t, err)
}

func TestOptionalInt(t *testing.T) {
	t.Parallel()

	i, err := OptionalInt(map[string]any{"key": 42}, "key", 0)
	require.NoError(t, err)
	assert.Equal(t, 42, i)

	i, err = OptionalInt(map[string]any{"key": int32(42)}, "key", 0)
	require.NoError(t, err)
	assert.Equal(t, 42, i)

	i, err = OptionalInt(map[string]any{"key": int64(42)}, "key", 0)
	require.NoError(t, err)
	assert.Equal(t, 42, i)

	i, err = OptionalInt(map[string]any{"key": 42.0}, "key", 0)
	require.NoError(t, err)
	assert.Equal(t, 42, i)

	_, err = OptionalInt(map[string]any{"key": 42.5}, "key", 0)
	require.Error(t, err)

	_, err = OptionalInt(map[string]any{"key": "42"}, "key", 0)
	require.Error(t, err)
}

func TestNormalizeLimit(t *testing.T) {
	t.Parallel()

	limit, err := NormalizeLimit(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 100, limit)

	limit, err = NormalizeLimit(map[string]any{"limit": 50})
	require.NoError(t, err)
	assert.Equal(t, 50, limit)

	limit, err = NormalizeLimit(map[string]any{"limit": 2000})
	require.NoError(t, err)
	assert.Equal(t, 1000, limit)

	_, err = NormalizeLimit(map[string]any{"limit": 0})
	require.Error(t, err)
}

func TestTaskFailure(t *testing.T) {
	t.Parallel()

	res, err := TaskFailure(assert.AnError)
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, assert.AnError.Error(), res.Error)
	assert.False(t, res.Timestamp.IsZero())
}

func TestSuccessResult(t *testing.T) {
	t.Parallel()

	res := SuccessResult(map[string]any{"key": "value"})
	assert.True(t, res.Success)
	assert.Equal(t, "value", res.Data.(map[string]any)["key"])
	assert.False(t, res.Timestamp.IsZero())
}
