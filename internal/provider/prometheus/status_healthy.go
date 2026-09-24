package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statusHealthyTask)(nil)

type statusHealthyTask struct {
	provider *Provider
}

func (t *statusHealthyTask) Name() string { return "prometheus.status.healthy" }

func (t *statusHealthyTask) JSONSchema() string { return statusHealthySchema }

func (t *statusHealthyTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	healthy, err := client.Healthy(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"healthy": healthy,
	}), nil
}

const statusHealthySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Healthy Status Parameters",
  "description": "Check Prometheus basic health.",
  "properties": {}
}`
