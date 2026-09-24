package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statusFlagsTask)(nil)

type statusFlagsTask struct {
	provider *Provider
}

func (t *statusFlagsTask) Name() string { return "prometheus.status.flags" }

func (t *statusFlagsTask) JSONSchema() string { return statusFlagsSchema }

func (t *statusFlagsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	flags, err := client.Flags(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{
		"flags": map[string]string(flags),
	}), nil
}

const statusFlagsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Flags Status Parameters",
  "description": "Get Prometheus command-line flags.",
  "properties": {}
}`
