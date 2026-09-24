package main

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthenticator(t *testing.T) {
	t.Parallel()

	auth, err := buildAuthenticator(config.Authentication{SecretKey: &config.SecretKeyAuth{Key: "token"}})
	require.NoError(t, err)
	assert.Equal(t, "secret_key", auth.Method())
}

func TestBuildAuthenticatorUnsupported(t *testing.T) {
	t.Parallel()

	_, err := buildAuthenticator(config.Authentication{MTLS: &config.MTLSAuth{CertFile: "cert", KeyFile: "key"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported auth method")
}

func TestExampleProviders(t *testing.T) {
	t.Parallel()

	providers := exampleProviders(config.Config{})
	require.Len(t, providers, 8)
	assert.Equal(t, "network", providers[0].Name())
	assert.Equal(t, "azure", providers[1].Name())
	assert.Equal(t, "kubernetes", providers[2].Name())
	assert.Equal(t, "prometheus", providers[3].Name())
	assert.Equal(t, "kafka", providers[4].Name())
	assert.Equal(t, "linux", providers[5].Name())
	assert.Equal(t, "log", providers[6].Name())
	assert.Equal(t, "os", providers[7].Name())
	assert.Len(t, providers[0].Capabilities(), 5)
	assert.Len(t, providers[3].Capabilities(), 20)
	assert.Len(t, providers[4].Capabilities(), 38)
	assert.Len(t, providers[5].Capabilities(), 74)
	assert.Len(t, providers[6].Capabilities(), 4)
}
