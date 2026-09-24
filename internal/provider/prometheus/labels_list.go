package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*labelsListTask)(nil)

type labelsListTask struct {
	provider *Provider
}

func (t *labelsListTask) Name() string { return "prometheus.labels.list" }

func (t *labelsListTask) JSONSchema() string { return labelsListSchema }

func (t *labelsListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	names, _, err := client.LabelNames(ctx, matches, start, end)
	if err != nil {
		return common.TaskFailure(err)
	}

	labels := make([]string, 0, len(names))
	for _, n := range names {
		labels = append(labels, string(n))
	}

	truncated := false
	if len(labels) > guardrails.MaxMetadataResults {
		labels = labels[:guardrails.MaxMetadataResults]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"labels":    labels,
		"count":     len(labels),
		"truncated": truncated,
	}), nil
}

const labelsListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Labels List Parameters",
  "description": "List known label names.",
  "properties": {
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
    }
  }
}`
