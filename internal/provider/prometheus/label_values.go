package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*labelValuesTask)(nil)

type labelValuesTask struct {
	provider *Provider
}

func (t *labelValuesTask) Name() string { return "prometheus.label.values" }

func (t *labelValuesTask) JSONSchema() string { return labelValuesSchema }

func (t *labelValuesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	label, err := common.RequireString(params, "label")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	matches, err := optionalStringSlice(params, "match")
	if err != nil {
		return common.TaskFailure(err)
	}

	start, end, err := parseOptionalRange(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	limit, err := common.OptionalInt(params, "limit", guardrails.MaxMetadataResults)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > guardrails.MaxMetadataResults {
		limit = guardrails.MaxMetadataResults
	}

	values, _, err := client.LabelValues(ctx, label, matches, start, end)
	if err != nil {
		return common.TaskFailure(err)
	}

	vals := make([]string, 0, len(values))
	for _, v := range values {
		vals = append(vals, string(v))
	}

	truncated := false
	if len(vals) > limit {
		vals = vals[:limit]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"values":    vals,
		"count":     len(vals),
		"truncated": truncated,
	}), nil
}

const labelValuesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Label Values Parameters",
  "description": "List values for a specific label name.",
  "properties": {
    "label": {
      "type": "string",
      "description": "Label name whose values to list."
    },
    "match": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Label matchers to filter by."
    },
    "start": {
      "type": "string",
      "format": "date-time",
      "description": "Start of the range in RFC3339."
    },
    "end": {
      "type": "string",
      "format": "date-time",
      "description": "End of the range in RFC3339."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of values to return."
    }
  },
  "required": ["label"]
}`
