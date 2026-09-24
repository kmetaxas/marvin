package linux

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// kernelSysctlGetTask implements the linux.kernel.sysctl.get capability.
type kernelSysctlGetTask struct{ provider *Provider }

const kernelSysctlGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.kernel.sysctl.get Parameters",
  "description": "Reads one or more sysctl values (explicit keys only).",
  "properties": {
    "key": {
      "type": "string",
      "description": "Single sysctl key to read (e.g. kernel.ostype)."
    },
    "keys": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Multiple sysctl keys to read."
    }
  },
  "oneOf": [
    { "required": ["key"] },
    { "required": ["keys"] }
  ]
}`

func (t *kernelSysctlGetTask) Name() string       { return "linux.kernel.sysctl.get" }
func (t *kernelSysctlGetTask) JSONSchema() string { return kernelSysctlGetSchema }

func (t *kernelSysctlGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("kernel.sysctl.get starting", "capability", t.Name())

	var keys []string
	if k, ok := params["key"]; ok {
		if s, ok := k.(string); ok && s != "" {
			keys = append(keys, s)
		}
	}
	if ks, ok := params["keys"]; ok {
		if arr, ok := ks.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					keys = append(keys, s)
				}
			}
		}
	}

	if len(keys) == 0 {
		return common.TaskFailure(fmt.Errorf("missing required parameter: key or keys"))
	}

	data, err := readKernelSysctlGet(t.provider.CurrentReader(), keys)
	if err != nil {
		slog.Info("kernel.sysctl.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("kernel.sysctl.get succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*kernelSysctlGetTask)(nil)

func readKernelSysctlGet(r *procfs.Reader, keys []string) (map[string]any, error) {
	results := make(map[string]string)
	var failures []string

	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		parts := strings.Split(key, ".")
		valid := true
		for _, p := range parts {
			if p == "" || p == ".." || strings.Contains(p, "/") || strings.Contains(p, "\\") {
				valid = false
				break
			}
		}
		if !valid {
			failures = append(failures, fmt.Sprintf("%s: invalid key", key))
			continue
		}

		val, err := r.ReadFileString(append([]string{"proc", "sys"}, parts...)...)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", key, err))
			continue
		}
		results[key] = strings.TrimSpace(val)
	}

	return map[string]any{
		"values":   results,
		"failures": failures,
		"count":    len(results),
	}, nil
}
