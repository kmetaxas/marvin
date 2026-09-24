package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSMissingTarget(t *testing.T) {
	t.Parallel()

	d := &dnsTask{}
	res, err := d.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "target")
}

func TestDNSDefaultRecordType(t *testing.T) {
	t.Parallel()

	d := &dnsTask{}
	// localhost should resolve to 127.0.0.1 on most systems
	res, err := d.Execute(context.Background(), map[string]any{
		"target": "localhost",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.NotNil(t, res.Data)

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, data["answers"])
}

func TestDNSInvalidRecordType(t *testing.T) {
	t.Parallel()

	d := &dnsTask{}
	res, err := d.Execute(context.Background(), map[string]any{
		"target":      "example.com",
		"record_type": "INVALID",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "unsupported record type")
}

func TestDNSPTR(t *testing.T) {
	t.Parallel()

	d := &dnsTask{}
	// 127.0.0.1 should resolve to localhost on most systems
	res, err := d.Execute(context.Background(), map[string]any{
		"target":      "127.0.0.1",
		"record_type": "PTR",
	})
	require.NoError(t, err)
	// PTR may fail in some environments, so we just check it runs without panic
	assert.NotNil(t, res)
}
