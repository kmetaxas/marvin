package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusConfigTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &statusConfigTask{}
	assert.Equal(t, "prometheus.status.config", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStatusConfigTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &statusConfigTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStatusConfigTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &statusConfigTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"redact_secrets": "yes"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "redact_secrets must be a boolean")
}

func TestStatusConfigTaskRedactsSecrets(t *testing.T) {
	t.Parallel()

	yamlStr := `
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: prometheus
    bearer_token: supersecret
    basic_auth:
      username: admin
      password: hunter2
    static_configs:
      - targets: ["localhost:9090"]
remote_write:
  - url: https://example.com/write
    authorization:
      credentials: topsecret
    oauth2:
      client_id: myclient
      client_secret: mysecret
`

	client := &fakePrometheusClient{
		configFunc: func(ctx context.Context) (v1.ConfigResult, error) {
			return v1.ConfigResult{YAML: yamlStr}, nil
		},
	}
	task := &statusConfigTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	cfg := data["config"].(map[string]any)

	global := cfg["global"].(map[string]any)
	assert.Equal(t, "15s", global["scrape_interval"])

	scrapeConfigs := cfg["scrape_configs"].([]any)
	sc := scrapeConfigs[0].(map[string]any)
	assert.Equal(t, "prometheus", sc["job_name"])
	assert.Equal(t, "[REDACTED]", sc["bearer_token"])

	basicAuth := sc["basic_auth"].(map[string]any)
	assert.Equal(t, "admin", basicAuth["username"])
	assert.Equal(t, "[REDACTED]", basicAuth["password"])

	remoteWrite := cfg["remote_write"].([]any)
	rw := remoteWrite[0].(map[string]any)
	authorization := rw["authorization"].(map[string]any)
	assert.Equal(t, "[REDACTED]", authorization["credentials"])

	oauth2 := rw["oauth2"].(map[string]any)
	assert.Equal(t, "[REDACTED]", oauth2["client_id"])
	assert.Equal(t, "[REDACTED]", oauth2["client_secret"])
}

func TestStatusConfigTaskNoRedact(t *testing.T) {
	t.Parallel()

	yamlStr := `
scrape_configs:
  - job_name: prometheus
    bearer_token: supersecret
`

	client := &fakePrometheusClient{
		configFunc: func(ctx context.Context) (v1.ConfigResult, error) {
			return v1.ConfigResult{YAML: yamlStr}, nil
		},
	}
	task := &statusConfigTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"redact_secrets": false})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	cfg := data["config"].(map[string]any)
	scrapeConfigs := cfg["scrape_configs"].([]any)
	sc := scrapeConfigs[0].(map[string]any)
	assert.Equal(t, "supersecret", sc["bearer_token"])
}

func TestStatusConfigTaskInvalidYAML(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		configFunc: func(ctx context.Context) (v1.ConfigResult, error) {
			return v1.ConfigResult{YAML: "\tbad: [unclosed"}, nil
		},
	}
	task := &statusConfigTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "parse prometheus config")
}

func TestStatusConfigTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		configFunc: func(ctx context.Context) (v1.ConfigResult, error) {
			return v1.ConfigResult{}, errors.New("config failed")
		},
	}
	task := &statusConfigTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "config failed")
}

func TestRedactConfigNestedSlices(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{
		"scrape_configs": []any{
			map[string]any{
				"job_name": "a",
				"basic_auth": map[string]any{
					"password": "p1",
				},
			},
			map[string]any{
				"job_name":          "b",
				"bearer_token_file": "/tmp/token",
			},
		},
		"token": "top",
	}

	out := redactConfig(cfg)

	assert.Equal(t, "[REDACTED]", out["token"])

	scrapeConfigs := out["scrape_configs"].([]any)
	first := scrapeConfigs[0].(map[string]any)
	assert.Equal(t, "a", first["job_name"])
	assert.Equal(t, "[REDACTED]", first["basic_auth"].(map[string]any)["password"])

	second := scrapeConfigs[1].(map[string]any)
	assert.Equal(t, "b", second["job_name"])
	assert.Equal(t, "[REDACTED]", second["bearer_token_file"])
}

func TestRedactConfigDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{
		"password": "secret",
		"nested": map[string]any{
			"token": "nested-secret",
		},
	}

	redactConfig(cfg)

	assert.Equal(t, "secret", cfg["password"])
	assert.Equal(t, "nested-secret", cfg["nested"].(map[string]any)["token"])
}
