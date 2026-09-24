package network

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	p := NewProvider()
	assert.Equal(t, "network", p.Name())
	assert.Len(t, p.Capabilities(), 5)
	assert.True(t, p.IsEnabled(nil))

	expected := []string{
		"network.dns.lookup",
		"network.icmp.echo_request",
		"network.socket.connect",
		"network.tls.handshake",
		"network.traceroute",
	}
	for _, name := range expected {
		found, ok := p.GetTask(name)
		require.True(t, ok, "expected task %q to exist", name)
		assert.NotNil(t, found)
		assert.Equal(t, name, found.Name())
	}
}

func TestProviderDefensiveCopy(t *testing.T) {
	t.Parallel()

	p := NewProvider()
	caps := p.Capabilities()
	require.Len(t, caps, 5)
	caps[0].Name = "tampered"
	assert.NotEqual(t, "tampered", p.Capabilities()[0].Name)
}

// Ensure all tasks satisfy the task.Task interface at compile time.
var _ task.Task = (*dnsTask)(nil)
var _ task.Task = (*pingTask)(nil)
var _ task.Task = (*socketTask)(nil)
var _ task.Task = (*tlsTask)(nil)
var _ task.Task = (*tracerouteTask)(nil)
