package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*appGatewayLogsTask)(nil)

type appGatewayLogsTask struct {
	provider *Provider
}

func (t *appGatewayLogsTask) Name() string { return "azure.logs.query_appgw" }

func (t *appGatewayLogsTask) JSONSchema() string { return appGWLogsSchema }

func (t *appGatewayLogsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	workspaceID, err := resolveWorkspaceID(t.provider.currentConfig(), params)
	if err != nil {
		return common.TaskFailure(err)
	}
	start, end, timespan, err := parseTimespan(params, "timespan", "PT1H")
	if err != nil {
		return common.TaskFailure(err)
	}
	maxRows, err := normalizeMaxRows(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	appGatewayName, err := common.OptionalString(params, "app_gateway_name", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentLogsClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure logs client is not configured"))
	}

	query := buildAppGatewayKQL(appGatewayName, maxRows)
	tables, err := client.QueryWorkspace(ctx, workspaceID, query, start, end)
	if err != nil {
		return common.TaskFailure(err)
	}
	rowCount := 0
	for _, table := range tables {
		rowCount += len(table.Rows)
	}
	return common.SuccessResult(map[string]any{"workspace_id": workspaceID, "timespan": timespan, "query": query, "tables": tables, "row_count": rowCount}), nil
}

func buildAppGatewayKQL(appGatewayName string, maxRows int) string {
	lines := []string{"AzureDiagnostics", "| where ResourceType == \"APPLICATIONGATEWAYS\""}
	if appGatewayName != "" {
		lines = append(lines, fmt.Sprintf("| where Resource == %q", appGatewayName))
	}
	lines = append(lines, "| project TimeGenerated, Resource, OperationName, Category, Message, clientIP_s, httpStatus_d, requestUri_s, host_s", fmt.Sprintf("| take %d", maxRows))
	return strings.Join(lines, "\n")
}

const appGWLogsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Application Gateway Log Query Parameters",
  "description": "Parameters for querying Azure Application Gateway logs from Log Analytics.",
  "properties": {
    "workspace_id": {
      "type": "string",
      "description": "Optional Log Analytics workspace ID. Uses provider config when omitted."
    },
    "timespan": {
      "type": "string",
      "description": "Relative timespan as ISO-8601 duration (for example PT1H) or Go duration string (for example 1h).",
      "default": "PT1H"
    },
    "max_rows": {
      "type": "integer",
      "description": "Maximum rows to return. Defaults to 100 and is capped at 1000.",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    },
    "app_gateway_name": {
      "type": "string",
      "description": "Optional Application Gateway resource name filter."
    }
  }
}`
