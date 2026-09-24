package network

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingMissingTarget(t *testing.T) {
	t.Parallel()

	p := &pingTask{}
	res, err := p.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "target")
}

func TestPingCountExceedsMax(t *testing.T) {
	t.Parallel()

	p := &pingTask{}
	res, err := p.Execute(context.Background(), map[string]any{
		"target": "127.0.0.1",
		"count":  31,
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "count exceeds maximum")
}

func TestPingCountAtMax(t *testing.T) {
	t.Parallel()

	p := &pingTask{}
	res, err := p.Execute(context.Background(), map[string]any{
		"target": "127.0.0.1",
		"count":  30,
	})
	require.NoError(t, err)
	// We don't assert Success here because ICMP may require privileges;
	// the important thing is it didn't fail due to count validation.
	assert.NotContains(t, res.Error, "count exceeds maximum")
}

func TestPingDefaultCount(t *testing.T) {
	t.Parallel()

	p := &pingTask{}
	res, err := p.Execute(context.Background(), map[string]any{
		"target": "127.0.0.1",
	})
	require.NoError(t, err)
	assert.NotContains(t, res.Error, "count exceeds maximum")
	if strings.Contains(res.Error, "permission denied") {
		t.Skip("ICMP ping requires unprivileged ping socket privileges; skipping in this environment")
	}
	assert.NotNil(t, res.Data)
}

func TestPingContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	p := &pingTask{}
	res, err := p.Execute(ctx, map[string]any{
		"target": "127.0.0.1",
		"count":  1,
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "context canceled")
}
