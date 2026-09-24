package autoconfig

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"tls.enabled": "flat wins",
		"tls": map[string]any{
			"enabled": true,
			"inner":   map[string]any{"name": "nested"},
		},
		"nil_value": nil,
	}

	v, ok := Lookup(raw, "tls.enabled")
	require.True(t, ok)
	assert.Equal(t, "flat wins", v)

	v, ok = Lookup(raw, "tls.inner.name")
	require.True(t, ok)
	assert.Equal(t, "nested", v)

	v, ok = Lookup(raw, "nil_value")
	require.True(t, ok)
	assert.Nil(t, v)

	_, ok = Lookup(raw, "tls.missing")
	assert.False(t, ok)

	_, ok = Lookup(nil, "tls.enabled")
	assert.False(t, ok)
}

func TestStringHelpers(t *testing.T) {
	t.Parallel()
	raw := map[string]any{"name": "marvin", "bad": 7}

	assert.Equal(t, "marvin", String(raw, "name"))
	v, ok := StringOK(raw, "name")
	require.True(t, ok)
	assert.Equal(t, "marvin", v)
	assert.Equal(t, "", String(raw, "missing"))
	_, ok = StringOK(raw, "bad")
	assert.False(t, ok)
	assert.Equal(t, "marvin", AsString("marvin"))
	_, ok = AsStringOK(12)
	assert.False(t, ok)
}

func TestBoolHelpers(t *testing.T) {
	t.Parallel()
	raw := map[string]any{"enabled": true, "bad": "true"}

	assert.True(t, Bool(raw, "enabled"))
	v, ok := BoolOK(raw, "enabled")
	require.True(t, ok)
	assert.True(t, v)
	assert.False(t, Bool(raw, "missing"))
	_, ok = BoolOK(raw, "bad")
	assert.False(t, ok)
	_, ok = AsBoolOK("true")
	assert.False(t, ok)
}

func TestIntHelpersAcceptIntegralValues(t *testing.T) {
	t.Parallel()
	cases := []any{
		int(42), int8(42), int16(42), int32(42), int64(42),
		uint(42), uint8(42), uint16(42), uint32(42), uint64(42),
		float32(42), float64(42),
	}

	for _, input := range cases {
		v, ok := AsIntOK(input)
		require.Truef(t, ok, "%T", input)
		assert.Equal(t, 42, v)
	}

	raw := map[string]any{"count": float64(42), "big": int64(42)}
	assert.Equal(t, 42, Int(raw, "count"))
	v, ok := IntOK(raw, "count")
	require.True(t, ok)
	assert.Equal(t, 42, v)
	i64, ok := Int64OK(raw, "big")
	require.True(t, ok)
	assert.Equal(t, int64(42), i64)
	assert.Equal(t, 0, AsInt("bad"))
}

func TestIntHelpersRejectNonIntegralAndOverflow(t *testing.T) {
	t.Parallel()
	rejected := []any{
		float64(42.5), float32(42.5), math.NaN(), math.Inf(1), math.Inf(-1),
		uint64(math.MaxUint64), "42",
	}

	for _, input := range rejected {
		_, ok := AsIntOK(input)
		assert.Falsef(t, ok, "%T %[1]v", input)
	}
}

func TestDurationHelpers(t *testing.T) {
	t.Parallel()
	cases := map[any]time.Duration{
		30 * time.Second: 30 * time.Second,
		"30s":            30 * time.Second,
		int(30):          30 * time.Second,
		int64(30):        30 * time.Second,
		float64(1.5):     1500 * time.Millisecond,
		float32(2.5):     2500 * time.Millisecond,
	}

	for input, want := range cases {
		got, ok := AsDurationOK(input)
		require.Truef(t, ok, "%T", input)
		assert.Equal(t, want, got)
	}

	raw := map[string]any{"timeout": "5s"}
	assert.Equal(t, 5*time.Second, Duration(raw, "timeout"))
	got, ok := DurationOK(raw, "timeout")
	require.True(t, ok)
	assert.Equal(t, 5*time.Second, got)
}

func TestDurationHelpersRejectInvalid(t *testing.T) {
	t.Parallel()
	rejected := []any{"not-duration", int(-1), int64(math.MaxInt64), float64(-1), math.NaN(), math.Inf(1)}

	for _, input := range rejected {
		_, ok := AsDurationOK(input)
		assert.Falsef(t, ok, "%T %[1]v", input)
	}
}

func TestStringSliceHelpers(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"strings": []string{"a", "b"},
		"any":     []any{"c", "d"},
		"empty":   []any{},
		"nil":     nil,
		"bad":     []any{"a", 1},
	}

	assert.Equal(t, []string{"a", "b"}, StringSlice(raw, "strings"))
	v, ok := StringSliceOK(raw, "any")
	require.True(t, ok)
	assert.Equal(t, []string{"c", "d"}, v)
	v, ok = StringSliceOK(raw, "empty")
	require.True(t, ok)
	assert.Empty(t, v)
	_, ok = StringSliceOK(raw, "nil")
	assert.False(t, ok)
	_, ok = StringSliceOK(raw, "bad")
	assert.False(t, ok)
}

func TestMapHelpers(t *testing.T) {
	t.Parallel()
	nested := map[string]any{"value": true}
	raw := map[string]any{"nested": nested, "bad": "value"}

	assert.Equal(t, nested, Map(raw, "nested"))
	got, ok := MapOK(raw, "nested")
	require.True(t, ok)
	assert.Equal(t, nested, got)
	_, ok = MapOK(raw, "bad")
	assert.False(t, ok)
}
