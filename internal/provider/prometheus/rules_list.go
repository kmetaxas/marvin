package prometheus

import (
	"context"
	"fmt"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*rulesListTask)(nil)

type rulesListTask struct {
	provider *Provider
}

func (t *rulesListTask) Name() string { return "prometheus.rules.list" }

func (t *rulesListTask) JSONSchema() string { return rulesListSchema }

func (t *rulesListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	ruleType, err := common.OptionalString(params, "type", "all")
	if err != nil {
		return common.TaskFailure(err)
	}
	switch ruleType {
	case "alert", "record", "all":
	default:
		return common.TaskFailure(fmt.Errorf("parameter type must be one of alert, record, all"))
	}

	ruleGroup, err := common.OptionalString(params, "rule_group", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	file, err := common.OptionalString(params, "file", "")
	if err != nil {
		return common.TaskFailure(err)
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

	result, err := client.Rules(ctx, nil)
	if err != nil {
		return common.TaskFailure(err)
	}

	rules := make([]map[string]any, 0)
	for _, group := range result.Groups {
		if ruleGroup != "" && group.Name != ruleGroup {
			continue
		}
		if file != "" && group.File != file {
			continue
		}
		for _, r := range group.Rules {
			item, ok := flattenRule(r, ruleType, matchLabels)
			if !ok {
				continue
			}
			item["rule_group"] = group.Name
			item["file"] = group.File
			item["interval"] = group.Interval
			rules = append(rules, item)
		}
	}

	truncated := false
	if len(rules) > limit {
		rules = rules[:limit]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"rules":     rules,
		"count":     len(rules),
		"truncated": truncated,
	}), nil
}

func flattenRule(r any, ruleType string, matchLabels map[string]string) (map[string]any, bool) {
	switch rule := r.(type) {
	case v1.AlertingRule:
		if ruleType != "all" && ruleType != "alert" {
			return nil, false
		}
		if len(matchLabels) > 0 && !labelsMatch(rule.Labels, matchLabels) {
			return nil, false
		}
		return map[string]any{
			"name":            rule.Name,
			"type":            "alert",
			"query":           rule.Query,
			"duration":        rule.Duration,
			"labels":          labelSetToMap(rule.Labels),
			"annotations":     labelSetToMap(rule.Annotations),
			"health":          string(rule.Health),
			"last_error":      rule.LastError,
			"evaluation_time": rule.EvaluationTime,
			"last_evaluation": formatTime(rule.LastEvaluation),
			"state":           rule.State,
		}, true
	case v1.RecordingRule:
		if ruleType != "all" && ruleType != "record" {
			return nil, false
		}
		if len(matchLabels) > 0 && !labelsMatch(rule.Labels, matchLabels) {
			return nil, false
		}
		return map[string]any{
			"name":            rule.Name,
			"type":            "record",
			"query":           rule.Query,
			"labels":          labelSetToMap(rule.Labels),
			"health":          string(rule.Health),
			"last_error":      rule.LastError,
			"evaluation_time": rule.EvaluationTime,
			"last_evaluation": formatTime(rule.LastEvaluation),
		}, true
	default:
		return nil, false
	}
}

const rulesListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Rules List Parameters",
  "description": "List alerting and recording rules.",
  "properties": {
    "type": {
      "type": "string",
      "enum": ["alert", "record", "all"],
      "default": "all",
      "description": "Filter rules by type."
    },
    "rule_group": {
      "type": "string",
      "description": "Filter by rule group name."
    },
    "file": {
      "type": "string",
      "description": "Filter by rule file path."
    },
    "match_labels": {
      "type": "object",
      "additionalProperties": { "type": "string" },
      "description": "Filter rules by label key/value pairs."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of rules to return."
    }
  }
}`
