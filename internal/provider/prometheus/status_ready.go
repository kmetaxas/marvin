package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statusReadyTask)(nil)

type statusReadyTask struct {
	provider *Provider
}

func (t *statusReadyTask) Name() string { return "prometheus.status.ready" }

func (t *statusReadyTask) JSONSchema() string { return statusReadySchema }

func (t *statusReadyTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	ready, err := client.Ready(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"ready": ready,
	}), nil
}

const statusReadySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Ready Status Parameters",
  "description": "Check whether Prometheus is ready to serve queries.",
  "properties": {}
}`
