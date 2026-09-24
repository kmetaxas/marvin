package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// processEnvironmentTask implements the linux.process.environment capability.
type processEnvironmentTask struct{ provider *Provider }

const processEnvironmentSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.process.environment Parameters",
  "description": "Parameters for linux.process.environment.",
  "properties": {
    "pid": {
      "type": "integer",
      "description": "Process ID"
    }
  },
  "required": ["pid"]
}`

func (t *processEnvironmentTask) Name() string       { return "linux.process.environment" }
func (t *processEnvironmentTask) JSONSchema() string { return processEnvironmentSchema }

func (t *processEnvironmentTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("process.environment starting", "capability", t.Name())

	pid, err := common.OptionalInt(params, "pid", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := ValidatePID(pid); err != nil {
		return common.TaskFailure(err)
	}

	env, err := readProcessEnvironment(t.provider.CurrentReader(), pid, t.provider.CurrentConfig())
	if err != nil {
		slog.Info("process.environment failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("process.environment succeeded", "capability", t.Name(), "vars", len(env))
	return common.SuccessResult(map[string]any{"environment": env}), nil
}

var _ task.Task = (*processEnvironmentTask)(nil)

func readProcessEnvironment(r *procfs.Reader, pid int, cfg config.LinuxConfig) (map[string]string, error) {
	pidStr := strconv.Itoa(pid)
	data, err := r.ReadFileString("proc", pidStr, "environ")
	if err != nil {
		return nil, fmt.Errorf("read environ: %w", err)
	}

	env := ParseEnvFile(data)

	// Mandatory secret redaction
	if cfg.RedactSecrets == nil || *cfg.RedactSecrets {
		patterns := cfg.SecretPatterns
		if patterns == nil {
			patterns = DefaultSecretPatterns()
		}
		env = RedactSecrets(env, patterns)
	}

	return env, nil
}
