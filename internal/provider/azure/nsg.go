package azure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*nsgTask)(nil)

type nsgTask struct{ provider *Provider }

func (t *nsgTask) Name() string { return "azure.network.nsg.list" }

func (t *nsgTask) JSONSchema() string { return nsgSchema }

func (t *nsgTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	resourceGroup, err := common.OptionalString(params, "resource_group", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	includeRules, err := common.OptionalBool(params, "include_rules", true)
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentNSGClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure network security group client is not configured"))
	}

	var items []securityGroupInfo
	if resourceGroup == "" {
		items, err = client.ListAll(ctx)
	} else {
		items, err = client.ListByResourceGroup(ctx, resourceGroup)
	}
	if err != nil {
		return common.TaskFailure(err)
	}
	if !includeRules {
		for idx := range items {
			items[idx].Rules = nil
		}
	}
	return common.SuccessResult(map[string]any{"network_security_groups": items, "count": len(items)}), nil
}

const nsgSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Network Security Group List Parameters",
  "description": "Parameters for listing Azure network security groups.",
  "properties": {
    "resource_group": {
      "type": "string",
      "description": "Optional Azure resource group. When omitted, lists across the subscription."
    },
    "include_rules": {
      "type": "boolean",
      "description": "Whether to include NSG rules in the response.",
      "default": true
    }
  }
}`

func (t *nsgTask) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"name": t.Name()})
}
