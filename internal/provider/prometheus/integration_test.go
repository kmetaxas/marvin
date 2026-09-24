package prometheus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPrometheusContainer launches a real prom/prometheus container that
// scrapes itself (using the image's default config) and returns the mapped
// host:port URL. The container is registered for cleanup via t.Cleanup.
func startPrometheusContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "prom/prometheus:latest",
		ExposedPorts: []string{"9090/tcp"},
		// The default image config (/etc/prometheus/prometheus.yml) already
		// scrapes Prometheus itself, generating metrics such as `up` and
		// `prometheus_build_info`. We pass the flags explicitly for clarity.
		Cmd: []string{
			"--config.file=/etc/prometheus/prometheus.yml",
			"--storage.tsdb.path=/prometheus",
			"--web.enable-lifecycle",
		},
		WaitingFor: wait.ForHTTP("/-/ready").
			WithPort("9090/tcp").
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start prometheus container")

	t.Cleanup(func() {
		terminateCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := container.Terminate(terminateCtx); err != nil {
			t.Logf("failed to terminate prometheus container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err, "failed to get container host")

	mappedPort, err := container.MappedPort(ctx, "9090/tcp")
	require.NoError(t, err, "failed to get mapped port")

	return fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
}

// newIntegrationProvider starts a container and returns a provider wired to it.
func newIntegrationProvider(t *testing.T) (provider.Provider, string) {
	t.Helper()
	url := startPrometheusContainer(t)
	p := NewProvider(config.PrometheusConfig{URL: url})
	return p, url
}

// execTask fetches a task by name and executes it with the given params,
// failing the test if the task is missing or execution returns an error.
func execTask(t *testing.T, p provider.Provider, name string, params map[string]any) task.Result {
	t.Helper()
	tk, ok := p.GetTask(name)
	require.True(t, ok, "task %q should exist", name)

	res, err := tk.Execute(context.Background(), params)
	require.NoError(t, err, "task %q returned an error", name)
	return res
}

// dataMap asserts the result is successful and returns its Data as a map.
func dataMap(t *testing.T, res task.Result, name string) map[string]any {
	t.Helper()
	require.True(t, res.Success, "task %q should succeed, got error: %s", name, res.Error)
	m, ok := res.Data.(map[string]any)
	require.True(t, ok, "task %q data should be a map, got %T", name, res.Data)
	return m
}

// waitForData polls the instant query until the `up` metric has at least one
// sample. Prometheus becomes "ready" before its first self-scrape completes,
// so this ensures subsequent queries observe real data.
func waitForData(t *testing.T, p provider.Provider) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		tk, ok := p.GetTask("prometheus.query.instant")
		require.True(t, ok, "task prometheus.query.instant should exist")
		res, err := tk.Execute(context.Background(), map[string]any{"query": "up"})
		if err == nil && res.Success {
			if m, ok := res.Data.(map[string]any); ok {
				if items, ok := m["result"].([]map[string]any); ok && len(items) > 0 {
					return
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("timed out waiting for Prometheus to scrape itself")
}

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p, _ := newIntegrationProvider(t)

	waitForData(t, p)

	now := time.Now()
	start := now.Add(-5 * time.Minute).Format(time.RFC3339)
	end := now.Format(time.RFC3339)

	t.Run("query.instant", func(t *testing.T) {
		res := execTask(t, p, "prometheus.query.instant", map[string]any{"query": "up"})
		m := dataMap(t, res, "prometheus.query.instant")

		assert.Equal(t, "vector", m["result_type"])
		assert.Contains(t, m, "result")
		assert.IsType(t, []map[string]any{}, m["result"])
		assert.Contains(t, m, "truncated")
		assert.IsType(t, false, m["truncated"])
		assert.Contains(t, m, "warnings")
		assert.IsType(t, []string{}, m["warnings"])

		// The self-scrape target should produce at least one `up` series.
		items := m["result"].([]map[string]any)
		assert.NotEmpty(t, items, "expected at least one `up` series")
	})

	t.Run("query.range", func(t *testing.T) {
		// The container's clock can run slightly ahead of the host clock, so a
		// range query bounded by host-computed `end` may initially miss the
		// freshly-scraped sample. Poll with a freshly-computed window until the
		// range query returns data.
		var m map[string]any
		deadline := time.Now().Add(60 * time.Second)
		for {
			now := time.Now()
			res := execTask(t, p, "prometheus.query.range", map[string]any{
				"query": "up",
				"start": now.Add(-5 * time.Minute).Format(time.RFC3339),
				"end":   now.Format(time.RFC3339),
				"step":  "15s",
			})
			m = dataMap(t, res, "prometheus.query.range")
			if items, ok := m["result"].([]map[string]any); ok && len(items) > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for range query to return data")
			}
			time.Sleep(500 * time.Millisecond)
		}

		assert.Equal(t, "matrix", m["result_type"])
		assert.Contains(t, m, "result")
		assert.IsType(t, []map[string]any{}, m["result"])
		assert.Contains(t, m, "truncated")
		assert.Contains(t, m, "warnings")

		items := m["result"].([]map[string]any)
		assert.NotEmpty(t, items, "expected at least one matrix stream")
		first := items[0]
		assert.Contains(t, first, "metric")
		assert.Contains(t, first, "values")
	})

	t.Run("series.list", func(t *testing.T) {
		res := execTask(t, p, "prometheus.series.list", map[string]any{"match": []string{"up"}})
		m := dataMap(t, res, "prometheus.series.list")

		assert.Contains(t, m, "series")
		assert.IsType(t, []map[string]string{}, m["series"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")

		series := m["series"].([]map[string]string)
		assert.NotEmpty(t, series, "expected at least one series matching `up`")
	})

	t.Run("labels.list", func(t *testing.T) {
		res := execTask(t, p, "prometheus.labels.list", map[string]any{})
		m := dataMap(t, res, "prometheus.labels.list")

		assert.Contains(t, m, "labels")
		assert.IsType(t, []string{}, m["labels"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")

		labels := m["labels"].([]string)
		assert.Contains(t, labels, "__name__", "expected __name__ label")
		assert.Contains(t, labels, "job", "expected job label")
	})

	t.Run("label.values", func(t *testing.T) {
		res := execTask(t, p, "prometheus.label.values", map[string]any{"label": "__name__"})
		m := dataMap(t, res, "prometheus.label.values")

		assert.Contains(t, m, "values")
		assert.IsType(t, []string{}, m["values"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")

		values := m["values"].([]string)
		assert.Contains(t, values, "up", "expected `up` metric name")
	})

	t.Run("metadata.get", func(t *testing.T) {
		res := execTask(t, p, "prometheus.metadata.get", map[string]any{"metric": "up"})
		m := dataMap(t, res, "prometheus.metadata.get")

		assert.Contains(t, m, "metadata")
		assert.IsType(t, []map[string]any{}, m["metadata"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")
	})

	t.Run("metric.names", func(t *testing.T) {
		res := execTask(t, p, "prometheus.metric.names", map[string]any{})
		m := dataMap(t, res, "prometheus.metric.names")

		assert.Contains(t, m, "names")
		assert.IsType(t, []string{}, m["names"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")

		names := m["names"].([]string)
		assert.Contains(t, names, "up", "expected `up` metric name")
		assert.Contains(t, names, "prometheus_build_info", "expected prometheus_build_info metric")
	})

	t.Run("alerts.list", func(t *testing.T) {
		res := execTask(t, p, "prometheus.alerts.list", map[string]any{})
		m := dataMap(t, res, "prometheus.alerts.list")

		assert.Contains(t, m, "alerts")
		assert.IsType(t, []map[string]any{}, m["alerts"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")
	})

	t.Run("rules.list", func(t *testing.T) {
		res := execTask(t, p, "prometheus.rules.list", map[string]any{})
		m := dataMap(t, res, "prometheus.rules.list")

		assert.Contains(t, m, "rules")
		assert.IsType(t, []map[string]any{}, m["rules"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")
	})

	t.Run("targets.list", func(t *testing.T) {
		res := execTask(t, p, "prometheus.targets.list", map[string]any{})
		m := dataMap(t, res, "prometheus.targets.list")

		assert.Contains(t, m, "targets")
		assert.IsType(t, []map[string]any{}, m["targets"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")

		targets := m["targets"].([]map[string]any)
		assert.NotEmpty(t, targets, "expected at least the self-scrape target")
	})

	t.Run("target.get", func(t *testing.T) {
		res := execTask(t, p, "prometheus.target.get", map[string]any{"job": "prometheus"})
		m := dataMap(t, res, "prometheus.target.get")

		assert.Contains(t, m, "job")
		assert.Equal(t, "prometheus", m["job"])
		assert.Contains(t, m, "instance")
		assert.Contains(t, m, "health")
		assert.Contains(t, m, "scrape_url")
	})

	t.Run("target.metadata", func(t *testing.T) {
		res := execTask(t, p, "prometheus.target.metadata", map[string]any{
			"match_target": map[string]string{"job": "prometheus"},
		})
		m := dataMap(t, res, "prometheus.target.metadata")

		assert.Contains(t, m, "metadata")
		assert.IsType(t, []map[string]any{}, m["metadata"])
		assert.Contains(t, m, "count")
		assert.Contains(t, m, "truncated")
	})

	t.Run("scrape_pools.list", func(t *testing.T) {
		res := execTask(t, p, "prometheus.scrape_pools.list", map[string]any{})
		m := dataMap(t, res, "prometheus.scrape_pools.list")

		assert.Contains(t, m, "pools")
		assert.IsType(t, []map[string]any{}, m["pools"])
		assert.Contains(t, m, "count")

		pools := m["pools"].([]map[string]any)
		assert.NotEmpty(t, pools, "expected at least the prometheus scrape pool")
	})

	t.Run("exemplars.query", func(t *testing.T) {
		res := execTask(t, p, "prometheus.exemplars.query", map[string]any{
			"query": "up",
			"start": start,
			"end":   end,
		})
		m := dataMap(t, res, "prometheus.exemplars.query")

		assert.Contains(t, m, "exemplars")
		assert.IsType(t, []map[string]any{}, m["exemplars"])
		assert.Contains(t, m, "count")
	})

	t.Run("status.runtime", func(t *testing.T) {
		res := execTask(t, p, "prometheus.status.runtime", map[string]any{})
		m := dataMap(t, res, "prometheus.status.runtime")

		for _, key := range []string{
			"version", "build_revision", "build_branch", "build_date",
			"go_version", "start_time", "storage_retention",
		} {
			assert.Contains(t, m, key, "expected key %q", key)
		}
		assert.NotEmpty(t, m["version"], "version should not be empty")
	})

	t.Run("status.tsdb", func(t *testing.T) {
		res := execTask(t, p, "prometheus.status.tsdb", map[string]any{})
		m := dataMap(t, res, "prometheus.status.tsdb")

		assert.Contains(t, m, "head_stats")
		assert.IsType(t, map[string]any{}, m["head_stats"])
		assert.Contains(t, m, "series_count_by_metric_name")
		assert.Contains(t, m, "label_value_count_by_label_name")
		assert.Contains(t, m, "memory_in_bytes_by_label_name")
		assert.Contains(t, m, "series_count_by_label_value_pair")
		assert.Contains(t, m, "truncated")
	})

	t.Run("status.flags", func(t *testing.T) {
		res := execTask(t, p, "prometheus.status.flags", map[string]any{})
		m := dataMap(t, res, "prometheus.status.flags")

		assert.Contains(t, m, "flags")
		flags, ok := m["flags"].(map[string]string)
		require.True(t, ok, "flags should be a map of strings, got %T", m["flags"])
		assert.NotEmpty(t, flags, "expected non-empty flags")
	})

	t.Run("status.config", func(t *testing.T) {
		res := execTask(t, p, "prometheus.status.config", map[string]any{})
		m := dataMap(t, res, "prometheus.status.config")

		assert.Contains(t, m, "config")
		cfg, ok := m["config"].(map[string]any)
		require.True(t, ok, "config should be a map, got %T", m["config"])
		assert.NotEmpty(t, cfg, "expected non-empty config")
	})

	t.Run("status.ready", func(t *testing.T) {
		res := execTask(t, p, "prometheus.status.ready", map[string]any{})
		m := dataMap(t, res, "prometheus.status.ready")

		assert.Contains(t, m, "ready")
		ready, ok := m["ready"].(bool)
		require.True(t, ok, "ready should be a bool, got %T", m["ready"])
		assert.True(t, ready, "prometheus should be ready")
	})

	t.Run("status.healthy", func(t *testing.T) {
		res := execTask(t, p, "prometheus.status.healthy", map[string]any{})
		m := dataMap(t, res, "prometheus.status.healthy")

		assert.Contains(t, m, "healthy")
		healthy, ok := m["healthy"].(bool)
		require.True(t, ok, "healthy should be a bool, got %T", m["healthy"])
		assert.True(t, healthy, "prometheus should be healthy")
	})
}
