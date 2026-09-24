package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statusRuntimeTask)(nil)

type statusRuntimeTask struct {
	provider *Provider
}

func (t *statusRuntimeTask) Name() string { return "prometheus.status.runtime" }

func (t *statusRuntimeTask) JSONSchema() string { return statusRuntimeSchema }

func (t *statusRuntimeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	info, err := client.Runtimeinfo(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	build, err := client.Buildinfo(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"version":           build.Version,
		"build_revision":    build.Revision,
		"build_branch":      build.Branch,
		"build_date":        build.BuildDate,
		"go_version":        build.GoVersion,
		"start_time":        formatTime(info.StartTime),
		"storage_retention": info.StorageRetention,
	}), nil
}

const statusRuntimeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Runtime Status Parameters",
  "description": "Get Prometheus runtime and build information.",
  "properties": {}
}`
