package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*activityChangesTask)(nil)

type activityChangesTask struct{ provider *Provider }

func (t *activityChangesTask) Name() string { return "azure.activity.changes" }

func (t *activityChangesTask) JSONSchema() string { return activitySchema }

func (t *activityChangesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start, _, timespan, err := parseTimespan(params, "timespan", "PT1H")
	if err != nil {
		return common.TaskFailure(err)
	}
	maxRows, err := normalizeMaxRows(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	resourceGroup, err := common.OptionalString(params, "resource_group", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	resourceType, err := common.OptionalString(params, "resource_type", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	operationName, err := common.OptionalString(params, "operation_name", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentActivityClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("azure activity logs client is not configured"))
	}

	filter := buildActivityFilter(start, resourceGroup, resourceType, operationName)
	events, err := client.List(ctx, filter)
	if err != nil {
		return common.TaskFailure(err)
	}
	if len(events) > maxRows {
		events = events[:maxRows]
	}
	return common.SuccessResult(map[string]any{"timespan": timespan, "filter": filter, "events": events, "count": len(events)}), nil
}

func buildActivityFilter(start time.Time, resourceGroup, resourceType, operationName string) string {
	parts := []string{fmt.Sprintf("eventTimestamp ge '%s'", start.Format(time.RFC3339))}
	if resourceGroup != "" {
		parts = append(parts, fmt.Sprintf("resourceGroupName eq '%s'", escapeSingleQuotes(resourceGroup)))
	}
	if resourceType != "" {
		parts = append(parts, fmt.Sprintf("resourceProvider eq '%s'", escapeSingleQuotes(resourceType)))
	}
	if operationName != "" {
		parts = append(parts, fmt.Sprintf("operationName/value eq '%s'", escapeSingleQuotes(operationName)))
	}
	return strings.Join(parts, " and ")
}

func escapeSingleQuotes(value string) string { return strings.ReplaceAll(value, "'", "''") }

const activitySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Azure Activity Change Parameters",
  "description": "Parameters for listing Azure activity log changes.",
  "properties": {
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
    "resource_group": {
      "type": "string",
      "description": "Optional resource group filter."
    },
    "resource_type": {
      "type": "string",
      "description": "Optional resource provider filter."
    },
    "operation_name": {
      "type": "string",
      "description": "Optional operation name filter."
    }
  }
}`
