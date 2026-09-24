package azure

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*appGatewayListenersTask)(nil)

type appGatewayListenersTask struct{ provider *Provider }

func (t *appGatewayListenersTask) Name() string { return "azure.network.appgw.listeners.list" }

func (t *appGatewayListenersTask) JSONSchema() string { return appGWListenersSchema }

func (t *appGatewayListenersTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
	listeners, err := client.GetListeners(ctx, resourceGroup, appGatewayName)
	if err != nil {
		return common.TaskFailure(err)
	}
	return common.SuccessResult(map[string]any{"resource_group": resourceGroup, "app_gateway_name": appGatewayName, "listeners": listeners, "count": len(listeners)}), nil
}

const appGWListenersSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Application Gateway Listener List Parameters",
  "description": "Parameters for listing Azure application gateway listeners.",
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
