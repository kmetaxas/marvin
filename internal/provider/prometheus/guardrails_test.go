package prometheus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultGuardrailPolicy(t *testing.T) {
	t.Parallel()

	p := DefaultGuardrailPolicy()
	assert.Equal(t, 168*time.Hour, p.MaxQueryRange)
	assert.Equal(t, 15*time.Second, p.MinStep)
	assert.Equal(t, 30*time.Second, p.QueryTimeout)
	assert.Equal(t, 1000, p.MaxReturnedSeries)
	assert.Equal(t, 5000, p.MaxReturnedSamples)
	assert.Equal(t, 1000, p.MaxMetadataResults)
	assert.Equal(t, 10*1024*1024, p.MaxResponseBytes)
}

func TestApplyDefaultsFillsZeroValues(t *testing.T) {
	t.Parallel()

	g := GuardrailPolicy{}
	g.ApplyDefaults()

	d := DefaultGuardrailPolicy()
	assert.Equal(t, d, g)
}

func TestApplyDefaultsPreservesNonZeroValues(t *testing.T) {
	t.Parallel()

	g := GuardrailPolicy{
		MaxQueryRange:      1 * time.Hour,
		MinStep:            5 * time.Second,
		QueryTimeout:       10 * time.Second,
		MaxReturnedSeries:  42,
		MaxReturnedSamples: 99,
		MaxMetadataResults: 7,
		MaxResponseBytes:   1234,
	}
	g.ApplyDefaults()

	assert.Equal(t, 1*time.Hour, g.MaxQueryRange)
	assert.Equal(t, 5*time.Second, g.MinStep)
	assert.Equal(t, 10*time.Second, g.QueryTimeout)
	assert.Equal(t, 42, g.MaxReturnedSeries)
	assert.Equal(t, 99, g.MaxReturnedSamples)
	assert.Equal(t, 7, g.MaxMetadataResults)
	assert.Equal(t, 1234, g.MaxResponseBytes)
}

func TestApplyDefaultsPartial(t *testing.T) {
	t.Parallel()

	g := GuardrailPolicy{MaxQueryRange: 2 * time.Hour}
	g.ApplyDefaults()

	d := DefaultGuardrailPolicy()
	assert.Equal(t, 2*time.Hour, g.MaxQueryRange)
	assert.Equal(t, d.MinStep, g.MinStep)
	assert.Equal(t, d.QueryTimeout, g.QueryTimeout)
	assert.Equal(t, d.MaxReturnedSeries, g.MaxReturnedSeries)
	assert.Equal(t, d.MaxReturnedSamples, g.MaxReturnedSamples)
	assert.Equal(t, d.MaxMetadataResults, g.MaxMetadataResults)
	assert.Equal(t, d.MaxResponseBytes, g.MaxResponseBytes)
}
