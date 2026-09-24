package log

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.GraylogConfig{URL: "https://graylog.example.com:9000/api"})
	assert.Equal(t, "log", p.Name())
	assert.Len(t, p.Capabilities(), 4)
	assert.True(t, p.IsEnabled(nil))

	expected := []string{
		"log.graylog.search",
		"log.graylog.count",
		"log.graylog.histogram",
		"log.graylog.fieldstats",
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

	p := NewProvider(config.GraylogConfig{URL: "https://graylog.example.com:9000/api"})
	caps := p.Capabilities()
	require.Len(t, caps, 4)
	caps[0].Name = "tampered"
	assert.NotEqual(t, "tampered", p.Capabilities()[0].Name)
}

func TestCapabilityDescription(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Search Graylog messages with filters, time range, and field selection", capabilityDescription("log.graylog.search"))
	assert.Equal(t, "Return the number of matching log messages without retrieving them", capabilityDescription("log.graylog.count"))
	assert.Equal(t, "Return time-bucketed hit counts for a query", capabilityDescription("log.graylog.histogram"))
	assert.Equal(t, "Return descriptive statistics (min/max/avg/sum) for a numeric field", capabilityDescription("log.graylog.fieldstats"))
	assert.Equal(t, "Graylog capability", capabilityDescription("log.graylog.unknown"))
}

func TestProviderUpdateConfigRecreatesClient(t *testing.T) {
	var created int
	origNewClient := newGraylogClient
	newGraylogClient = func(cfg config.GraylogConfig) graylogClient {
		created++
		return origNewClient(cfg)
	}
	defer func() { newGraylogClient = origNewClient }()

	p := NewProvider(config.GraylogConfig{URL: "https://old.example.com/api"})
	require.Equal(t, 1, created)

	p.UpdateConfig("log.graylog.search", map[string]any{"url": "https://old.example.com/api"})
	require.Equal(t, 1, created)

	p.UpdateConfig("log.graylog.search", map[string]any{"url": "https://new.example.com/api"})
	require.Equal(t, 2, created)

	p.UpdateConfig("log.graylog.search", map[string]any{"url": "https://new.example.com/api", "timeout": "60s"})
	require.Equal(t, 3, created)

	p.UpdateConfig("log.graylog.search", map[string]any{"url": "https://new.example.com/api", "timeout": "60s"})
	require.Equal(t, 3, created)
}

func TestProviderIsConfigured(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.GraylogConfig{URL: "https://graylog.example.com:9000/api"})
	assert.True(t, p.IsConfigured())

	p2 := NewProvider(config.GraylogConfig{})
	assert.False(t, p2.IsConfigured())
}

// Ensure all tasks satisfy the task.Task interface at compile time.
var _ task.Task = (*searchTask)(nil)
var _ task.Task = (*countTask)(nil)
var _ task.Task = (*histogramTask)(nil)
var _ task.Task = (*fieldstatsTask)(nil)
