package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*vnetFlowLogsTask)(nil)

type vnetFlowLogsTask struct {
	provider *Provider
}

func (t *vnetFlowLogsTask) Name() string { return "azure.logs.query_vnet_flow" }

func (t *vnetFlowLogsTask) JSONSchema() string { return vnetFlowLogsSchema }

func (t *vnetFlowLogsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
	nsgName, err := common.OptionalString(params, "nsg_name", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	sourceIP, err := common.OptionalString(params, "source_ip", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	destIP, err := common.OptionalString(params, "dest_ip", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentLogsClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure logs client is not configured"))
	}

	query := buildVNetFlowKQL(nsgName, sourceIP, destIP, maxRows)
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

func buildVNetFlowKQL(nsgName, sourceIP, destIP string, maxRows int) string {
	lines := []string{"AzureNetworkAnalytics_CL"}
	if nsgName != "" {
		lines = append(lines, fmt.Sprintf("| where NSG_s == %q", nsgName))
	}
	if sourceIP != "" {
		lines = append(lines, fmt.Sprintf("| where SrcIP_s == %q", sourceIP))
	}
	if destIP != "" {
		lines = append(lines, fmt.Sprintf("| where DestIP_s == %q", destIP))
	}
	lines = append(lines, "| project TimeGenerated, NSG_s, SrcIP_s, DestIP_s, DestPort_d, Protocol_s, FlowStatus_s, Direction_s", fmt.Sprintf("| take %d", maxRows))
	return strings.Join(lines, "\n")
}

const vnetFlowLogsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure VNet Flow Log Query Parameters",
  "description": "Parameters for querying Azure VNet flow logs from Log Analytics.",
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
    "nsg_name": {
      "type": "string",
      "description": "Optional NSG name filter."
    },
    "source_ip": {
      "type": "string",
      "description": "Optional source IP filter."
    },
    "dest_ip": {
      "type": "string",
      "description": "Optional destination IP filter."
    }
  }
}`
