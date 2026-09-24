package azure

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*udrTask)(nil)

type udrTask struct{ provider *Provider }

func (t *udrTask) Name() string { return "azure.network.udr.list" }

func (t *udrTask) JSONSchema() string { return udrSchema }

func (t *udrTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	resourceGroup, err := common.OptionalString(params, "resource_group", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	includeRoutes, err := common.OptionalBool(params, "include_routes", true)
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentUDRClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure route table client is not configured"))
	}

	var items []routeTableInfo
	if resourceGroup == "" {
		items, err = client.ListAll(ctx)
	} else {
		items, err = client.ListByResourceGroup(ctx, resourceGroup)
	}
	if err != nil {
		return common.TaskFailure(err)
	}
	if !includeRoutes {
		for idx := range items {
			items[idx].Routes = nil
		}
	}
	return common.SuccessResult(map[string]any{"route_tables": items, "count": len(items)}), nil
}

const udrSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Route Table List Parameters",
  "description": "Parameters for listing Azure route tables.",
  "properties": {
    "resource_group": {
      "type": "string",
      "description": "Optional Azure resource group. When omitted, lists across the subscription."
    },
    "include_routes": {
      "type": "boolean",
      "description": "Whether to include routes in the response.",
      "default": true
    }
  }
}`
