package postgres

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresClientInvalidConfig(t *testing.T) {
	t.Parallel()

	c := newPostgresClient(config.PostgresConfig{
		DSN: "://bad-url",
	})
	require.Error(t, c.initErr)
	assert.Contains(t, c.initErr.Error(), "parse connection string")
}

func TestBuildTLSConfigInvalidBase64(t *testing.T) {
	t.Parallel()

	_, err := buildTLSConfig(config.PostgresTLSConfig{
		Enabled: true,
		CAData:  "not-valid-base64!!!",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load CA")
}

func TestErrorRowScanReturnsError(t *testing.T) {
	t.Parallel()

	expectedErr := assert.AnError
	row := &errorRow{err: expectedErr}
	err := row.Scan()
	assert.Equal(t, expectedErr, err)
}
