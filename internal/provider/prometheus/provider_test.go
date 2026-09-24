package prometheus

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.PrometheusConfig{URL: "http://localhost:9090"})
	assert.Equal(t, "prometheus", p.Name())
	assert.Len(t, p.Capabilities(), 20)
	assert.True(t, p.IsEnabled(nil))

	expected := []string{
		"prometheus.query.instant",
		"prometheus.query.range",
		"prometheus.series.list",
		"prometheus.labels.list",
		"prometheus.label.values",
		"prometheus.metadata.get",
		"prometheus.metric.names",
		"prometheus.alerts.list",
		"prometheus.rules.list",
		"prometheus.targets.list",
		"prometheus.target.get",
		"prometheus.target.metadata",
		"prometheus.scrape_pools.list",
		"prometheus.exemplars.query",
		"prometheus.status.runtime",
		"prometheus.status.tsdb",
		"prometheus.status.flags",
		"prometheus.status.config",
		"prometheus.status.ready",
		"prometheus.status.healthy",
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

	p := NewProvider(config.PrometheusConfig{URL: "http://localhost:9090"})
	caps := p.Capabilities()
	require.Len(t, caps, 20)
	caps[0].Name = "tampered"
	assert.NotEqual(t, "tampered", p.Capabilities()[0].Name)
}

func TestCapabilityDescription(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Executes an instant PromQL query at a point in time", capabilityDescription("prometheus.query.instant"))
	assert.Equal(t, "Prometheus capability", capabilityDescription("prometheus.unknown.thing"))
}

func fakeClientProvider(client prometheusClient) *Provider {
	p := NewProvider(config.PrometheusConfig{URL: "http://localhost:9090"})
	cfg := p.state.Config()
	p.mu.Lock()
	p.state = autoconfig.NewState(&p.mu, cfg, client)
	p.mu.Unlock()
	return p
}

func TestProviderUpdateConfigRecreatesClient(t *testing.T) {
	var created int
	origNewClient := newPrometheusClient
	newPrometheusClient = func(cfg config.PrometheusConfig) *clientImpl {
		created++
		return origNewClient(cfg)
	}
	defer func() { newPrometheusClient = origNewClient }()

	p := NewProvider(config.PrometheusConfig{URL: "http://old:9090"})
	require.Equal(t, 1, created)

	p.UpdateConfig("prometheus.query.instant", map[string]any{"url": "http://old:9090"})
	require.Equal(t, 1, created)

	p.UpdateConfig("prometheus.query.instant", map[string]any{"url": "http://new:9090"})
	require.Equal(t, 2, created)

	p.UpdateConfig("prometheus.query.instant", map[string]any{"url": "http://new:9090", "timeout": "60s"})
	require.Equal(t, 3, created)

	p.UpdateConfig("prometheus.query.instant", map[string]any{"url": "http://new:9090", "timeout": "60s"})
	require.Equal(t, 3, created)
}

var _ task.Task = (*queryInstantTask)(nil)
var _ task.Task = (*queryRangeTask)(nil)
var _ task.Task = (*seriesListTask)(nil)
var _ task.Task = (*labelsListTask)(nil)
var _ task.Task = (*labelValuesTask)(nil)
var _ task.Task = (*metadataGetTask)(nil)
var _ task.Task = (*metricNamesTask)(nil)
var _ task.Task = (*alertsListTask)(nil)
var _ task.Task = (*rulesListTask)(nil)
var _ task.Task = (*targetsListTask)(nil)
var _ task.Task = (*targetGetTask)(nil)
var _ task.Task = (*targetMetadataTask)(nil)
var _ task.Task = (*scrapePoolsListTask)(nil)
var _ task.Task = (*exemplarsQueryTask)(nil)
var _ task.Task = (*statusRuntimeTask)(nil)
var _ task.Task = (*statusTsdbTask)(nil)
var _ task.Task = (*statusFlagsTask)(nil)
var _ task.Task = (*statusConfigTask)(nil)
var _ task.Task = (*statusReadyTask)(nil)
var _ task.Task = (*statusHealthyTask)(nil)
