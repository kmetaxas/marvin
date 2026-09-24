package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemEnvironmentTask returns selected system-level environment information.
type systemEnvironmentTask struct{ provider *Provider }

const systemEnvironmentSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "System Environment Parameters",
  "description": "Parameters for retrieving system environment variables with secret redaction.",
  "properties": {
    "names": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Optional list of environment variable names to retrieve."
    }
  }
}`

func (t *systemEnvironmentTask) Name() string       { return "linux.system.environment" }
func (t *systemEnvironmentTask) JSONSchema() string { return systemEnvironmentSchema }

func (t *systemEnvironmentTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("system.environment starting", "capability", t.Name())

	cfg := t.provider.CurrentConfig()

	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	if rawNames, ok := params["names"]; ok {
		names, err := toStringSlice(rawNames)
		if err != nil {
			return common.TaskFailure(fmt.Errorf("names parameter must be a string array: %w", err))
		}
		filtered := make(map[string]string)
		for _, name := range names {
			if val, ok := env[name]; ok {
				filtered[name] = val
			}
		}
		env = filtered
	}

	if cfg.RedactSecrets == nil || *cfg.RedactSecrets {
		env = RedactSecrets(env, cfg.SecretPatterns)
	}

	result := map[string]any{
		"environment": env,
		"count":       len(env),
		"redacted":    cfg.RedactSecrets == nil || *cfg.RedactSecrets,
	}

	slog.Info("system.environment succeeded", "capability", t.Name(), "count", len(env))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*systemEnvironmentTask)(nil)

func toStringSlice(v any) ([]string, error) {
	switch val := v.(type) {
	case []string:
		return val, nil
	case []any:
		var out []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			} else {
				return nil, fmt.Errorf("expected string, got %T", item)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected array, got %T", v)
	}
}
