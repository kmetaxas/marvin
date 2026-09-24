package azure

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*lbHealthTask)(nil)

type lbHealthTask struct{ provider *Provider }

func (t *lbHealthTask) Name() string { return "azure.network.lb.rule.health" }

func (t *lbHealthTask) JSONSchema() string { return lbHealthSchema }

func (t *lbHealthTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	resourceGroup, err := common.RequireString(params, "resource_group")
	if err != nil {
		return common.TaskFailure(err)
	}
	loadBalancerName, err := common.RequireString(params, "load_balancer_name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentLBProbeClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure load balancer probe client is not configured"))
	}
	probes, err := client.List(ctx, resourceGroup, loadBalancerName)
	if err != nil {
		return common.TaskFailure(err)
	}
	return common.SuccessResult(map[string]any{"resource_group": resourceGroup, "load_balancer_name": loadBalancerName, "probes": probes, "count": len(probes)}), nil
}

const lbHealthSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Load Balancer Probe Health Parameters",
  "description": "Parameters for listing Azure load balancer probe configuration.",
  "properties": {
    "resource_group": {
      "type": "string",
      "description": "Azure resource group containing the load balancer."
    },
    "load_balancer_name": {
      "type": "string",
      "description": "Azure load balancer name."
    }
  },
  "required": ["resource_group", "load_balancer_name"]
}`
