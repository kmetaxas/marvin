package prometheus

import (
	"net/http"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthRoundTripperNone(t *testing.T) {
	t.Parallel()

	base := http.DefaultTransport
	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{Type: "none"}, base)
	require.NoError(t, err)
	assert.Equal(t, base, rt)
}

func TestBuildAuthRoundTripperEmptyType(t *testing.T) {
	t.Parallel()

	base := http.DefaultTransport
	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{}, base)
	require.NoError(t, err)
	assert.Equal(t, base, rt)
}

func TestBuildAuthRoundTripperNilBase(t *testing.T) {
	t.Parallel()

	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{Type: "none"}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.DefaultTransport, rt)
}

func TestBuildAuthRoundTripperBasic(t *testing.T) {
	t.Parallel()

	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{
		Type:     "basic",
		Username: "user",
		Password: "pass",
	}, http.DefaultTransport)
	require.NoError(t, err)
	require.NotNil(t, rt)
	assert.NotEqual(t, http.DefaultTransport, rt)
}

func TestBuildAuthRoundTripperBearer(t *testing.T) {
	t.Parallel()

	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{
		Type:  "bearer",
		Token: "secret-token",
	}, http.DefaultTransport)
	require.NoError(t, err)
	require.NotNil(t, rt)
	assert.NotEqual(t, http.DefaultTransport, rt)
}

func TestBuildAuthRoundTripperHeader(t *testing.T) {
	t.Parallel()

	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{
		Type:        "header",
		HeaderName:  "X-Api-Key",
		HeaderValue: "abc123",
	}, http.DefaultTransport)
	require.NoError(t, err)
	require.NotNil(t, rt)
	assert.NotEqual(t, http.DefaultTransport, rt)
}

func TestBuildAuthRoundTripperUnsupported(t *testing.T) {
	t.Parallel()

	rt, err := buildAuthRoundTripper(config.PrometheusAuthConfig{Type: "oauth2"}, http.DefaultTransport)
	require.Error(t, err)
	assert.Nil(t, rt)
	assert.Contains(t, err.Error(), "unsupported auth type")
}
