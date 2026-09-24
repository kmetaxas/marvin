package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardrailsDefaults(t *testing.T) {
	t.Parallel()

	g := logGuardrails{}
	g.ApplyDefaults()
	assert.Equal(t, 24*time.Hour, g.MaxQueryRange)
	assert.Equal(t, 100, g.MaxResults)
	assert.Equal(t, 100, g.MaxHistogramBuckets)
	assert.Equal(t, 30*time.Second, g.QueryTimeout)
}

func TestGuardrailsValidateTimerangeRelative(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxQueryRange: time.Hour}

	tr := &timerange{Type: "relative", Range: 1800}
	require.NoError(t, g.ValidateTimerange(tr))

	tr2 := &timerange{Type: "relative", Range: 7200}
	err := g.ValidateTimerange(tr2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestGuardrailsValidateTimerangeAbsolute(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxQueryRange: time.Hour}

	tr := &timerange{Type: "absolute", From: "2024-01-01T00:00:00Z", To: "2024-01-01T00:30:00Z"}
	require.NoError(t, g.ValidateTimerange(tr))

	tr2 := &timerange{Type: "absolute", From: "2024-01-01T00:00:00Z", To: "2024-01-01T02:00:00Z"}
	err := g.ValidateTimerange(tr2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestGuardrailsValidateTimerangeInvalidFrom(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxQueryRange: time.Hour}
	tr := &timerange{Type: "absolute", From: "not-a-date", To: "2024-01-01T00:00:00Z"}
	err := g.ValidateTimerange(tr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RFC3339")
}

func TestGuardrailsValidateTimerangeKeyword(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxQueryRange: time.Hour}
	tr := &timerange{Type: "keyword", Keyword: "last five minutes"}
	require.NoError(t, g.ValidateTimerange(tr))
}

func TestGuardrailsValidateTimerangeNil(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxQueryRange: time.Hour}
	require.NoError(t, g.ValidateTimerange(nil))
}

func TestGuardrailsLimitResults(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxResults: 100}
	assert.Equal(t, 50, g.LimitResults(50))
	assert.Equal(t, 100, g.LimitResults(200))
	assert.Equal(t, 100, g.LimitResults(0))
	assert.Equal(t, 100, g.LimitResults(-1))
}

func TestGuardrailsLimitBuckets(t *testing.T) {
	t.Parallel()

	g := logGuardrails{MaxHistogramBuckets: 50}
	assert.Equal(t, 20, g.LimitBuckets(20))
	assert.Equal(t, 50, g.LimitBuckets(100))
	assert.Equal(t, 0, g.LimitBuckets(0))
}
