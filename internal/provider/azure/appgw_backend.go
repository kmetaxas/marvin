package azure

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*appGatewayBackendHealthTask)(nil)

type appGatewayBackendHealthTask struct{ provider *Provider }

func (t *appGatewayBackendHealthTask) Name() string { return "azure.network.appgw.backend.health" }

func (t *appGatewayBackendHealthTask) JSONSchema() string { return appGWBackendSchema }

func (t *appGatewayBackendHealthTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	resourceGroup, err := common.RequireString(params, "resource_group")
	if err != nil {
		return common.TaskFailure(err)
	}
	appGatewayName, err := common.RequireString(params, "app_gateway_name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentAppGatewayClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure application gateway client is not configured"))
	}
	pools, err := client.BackendHealth(ctx, resourceGroup, appGatewayName)
	if err != nil {
		return common.TaskFailure(err)
	}
	return common.SuccessResult(map[string]any{"resource_group": resourceGroup, "app_gateway_name": appGatewayName, "backend_pools": pools, "count": len(pools)}), nil
}

const appGWBackendSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Application Gateway Backend Health Parameters",
  "description": "Parameters for retrieving Azure application gateway backend health.",
  "properties": {
    "resource_group": {
      "type": "string",
      "description": "Azure resource group containing the application gateway."
    },
    "app_gateway_name": {
      "type": "string",
      "description": "Azure application gateway name."
    }
  },
  "required": ["resource_group", "app_gateway_name"]
}`
