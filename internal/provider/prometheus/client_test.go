package prometheus

import (
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusClientDefaults(t *testing.T) {
	t.Parallel()

	c := newPrometheusClient(config.PrometheusConfig{URL: "http://localhost:9090"})
	require.NoError(t, c.initErr)
	assert.Equal(t, "http://localhost:9090", c.baseURL)
	assert.NotNil(t, c.api)

	d := DefaultGuardrailPolicy()
	assert.Equal(t, d, c.guardrails)
}

func TestNewPrometheusClientCustomGuardrails(t *testing.T) {
	t.Parallel()

	cfg := config.PrometheusConfig{
		URL: "http://localhost:9090",
		Guardrails: config.PrometheusGuardrails{
			MaxQueryRange:      1 * time.Hour,
			MinStep:            5 * time.Second,
			QueryTimeout:       10 * time.Second,
			MaxReturnedSeries:  42,
			MaxReturnedSamples: 99,
			MaxMetadataResults: 7,
			MaxResponseBytes:   1234,
		},
	}
	c := newPrometheusClient(cfg)
	require.NoError(t, c.initErr)

	assert.Equal(t, 1*time.Hour, c.guardrails.MaxQueryRange)
	assert.Equal(t, 5*time.Second, c.guardrails.MinStep)
	assert.Equal(t, 10*time.Second, c.guardrails.QueryTimeout)
	assert.Equal(t, 42, c.guardrails.MaxReturnedSeries)
	assert.Equal(t, 99, c.guardrails.MaxReturnedSamples)
	assert.Equal(t, 7, c.guardrails.MaxMetadataResults)
	assert.Equal(t, 1234, c.guardrails.MaxResponseBytes)
}

func TestNewPrometheusClientInvalidURL(t *testing.T) {
	t.Parallel()

	c := newPrometheusClient(config.PrometheusConfig{URL: "://bad-url"})
	require.Error(t, c.initErr)
}

func TestNewPrometheusClientUnsupportedAuth(t *testing.T) {
	t.Parallel()

	c := newPrometheusClient(config.PrometheusConfig{
		URL:  "http://localhost:9090",
		Auth: config.PrometheusAuthConfig{Type: "oauth2"},
	})
	require.Error(t, c.initErr)
	assert.Contains(t, c.initErr.Error(), "auth transport")
}

func TestNewPrometheusClientBearerAuth(t *testing.T) {
	t.Parallel()

	c := newPrometheusClient(config.PrometheusConfig{
		URL:  "http://localhost:9090",
		Auth: config.PrometheusAuthConfig{Type: "bearer", Token: "tok"},
	})
	require.NoError(t, c.initErr)
	assert.NotNil(t, c.api)
}
