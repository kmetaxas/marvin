package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*alertsListTask)(nil)

type alertsListTask struct {
	provider *Provider
}

func (t *alertsListTask) Name() string { return "prometheus.alerts.list" }

func (t *alertsListTask) JSONSchema() string { return alertsListSchema }

func (t *alertsListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	state, err := common.OptionalString(params, "state", "all")
	if err != nil {
		return common.TaskFailure(err)
	}
	switch state {
	case "pending", "firing", "all":
	default:
		return common.TaskFailure(fmt.Errorf("parameter state must be one of pending, firing, all"))
	}

	matchLabels, err := optionalStringMap(params, "match_labels")
	if err != nil {
		return common.TaskFailure(err)
	}

	limit, err := common.OptionalInt(params, "limit", guardrails.MaxReturnedSeries)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > guardrails.MaxReturnedSeries {
		limit = guardrails.MaxReturnedSeries
	}

	result, err := client.Alerts(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	alerts := make([]map[string]any, 0, len(result.Alerts))
	for _, a := range result.Alerts {
		if state != "all" && string(a.State) != state {
			continue
		}
		if len(matchLabels) > 0 && !labelsMatch(a.Labels, matchLabels) {
			continue
		}
		alerts = append(alerts, map[string]any{
			"name":        string(a.Labels["alertname"]),
			"state":       string(a.State),
			"value":       a.Value,
			"active_at":   formatTime(a.ActiveAt),
			"labels":      labelSetToMap(a.Labels),
			"annotations": labelSetToMap(a.Annotations),
		})
	}

	truncated := false
	if len(alerts) > limit {
		alerts = alerts[:limit]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"alerts":    alerts,
		"count":     len(alerts),
		"truncated": truncated,
	}), nil
}

const alertsListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Alerts List Parameters",
  "description": "List active and pending alerts.",
  "properties": {
    "state": {
      "type": "string",
      "enum": ["pending", "firing", "all"],
      "default": "all",
      "description": "Filter alerts by state."
    },
    "match_labels": {
      "type": "object",
      "additionalProperties": { "type": "string" },
      "description": "Filter alerts by label key/value pairs."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of alerts to return."
    }
  }
}`
