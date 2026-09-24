package prometheus

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statusConfigTask)(nil)

type statusConfigTask struct {
	provider *Provider
}

func (t *statusConfigTask) Name() string { return "prometheus.status.config" }

func (t *statusConfigTask) JSONSchema() string { return statusConfigSchema }

func (t *statusConfigTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	redact, err := common.OptionalBool(params, "redact_secrets", true)
	if err != nil {
		return common.TaskFailure(err)
	}

	result, err := client.Config(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	cfg, err := parseConfigYAML(result.YAML)
	if err != nil {
		return common.TaskFailure(err)
	}

	if redact {
		cfg = redactConfig(cfg)
	}

	return common.SuccessResult(map[string]any{
		"config": cfg,
	}), nil
}

// parseConfigYAML parses a Prometheus config YAML string into a generic map.
func parseConfigYAML(yamlStr string) (map[string]any, error) {
	var cfg map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &cfg); err != nil {
		return nil, fmt.Errorf("parse prometheus config: %w", err)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

// sensitiveKeys is the set of config keys whose values must be redacted.
var sensitiveKeys = map[string]struct{}{
	"bearer_token":      {},
	"bearer_token_file": {},
	"password":          {},
	"password_file":     {},
	"credentials":       {},
	"credentials_file":  {},
	"client_secret":     {},
	"client_id":         {},
	"token":             {},
}

// redactConfig recursively walks a parsed config map and replaces the values of
// sensitive keys with "[REDACTED]". Nested maps and slices of maps are walked.
func redactConfig(cfg map[string]any) map[string]any {
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		out[k] = redactValue(v, k)
	}
	return out
}

func redactValue(v any, key string) any {
	if _, sensitive := sensitiveKeys[key]; sensitive {
		return "[REDACTED]"
	}
	switch val := v.(type) {
	case map[string]any:
		return redactConfig(val)
	case []any:
		out := make([]any, 0, len(val))
		for _, item := range val {
			out = append(out, redactValue(item, ""))
		}
		return out
	default:
		return v
	}
}

const statusConfigSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Config Status Parameters",
  "description": "Get the loaded Prometheus configuration (redacted).",
  "properties": {
    "redact_secrets": {
      "type": "boolean",
      "default": true,
      "description": "Whether to redact sensitive values from the config."
    }
  }
}`
