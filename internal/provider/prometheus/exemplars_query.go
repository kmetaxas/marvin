package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*exemplarsQueryTask)(nil)

type exemplarsQueryTask struct {
	provider *Provider
}

func (t *exemplarsQueryTask) Name() string { return "prometheus.exemplars.query" }

func (t *exemplarsQueryTask) JSONSchema() string { return exemplarsQuerySchema }

func (t *exemplarsQueryTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	query, err := common.RequireString(params, "query")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	startStr, err := common.RequireString(params, "start")
	if err != nil {
		return common.TaskFailure(err)
	}
	endStr, err := common.RequireString(params, "end")
	if err != nil {
		return common.TaskFailure(err)
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("parameter start must be RFC3339: %w", err))
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return common.TaskFailure(fmt.Errorf("parameter end must be RFC3339: %w", err))
	}
	if !end.After(start) {
		return common.TaskFailure(fmt.Errorf("parameter end must be after start"))
	}
	if end.Sub(start) > guardrails.MaxQueryRange {
		return common.TaskFailure(fmt.Errorf("query range exceeds maximum allowed (%s)", guardrails.MaxQueryRange))
	}

	results, err := client.QueryExemplars(ctx, query, start, end)
	if err != nil {
		return common.TaskFailure(err)
	}

	exemplars := make([]map[string]any, 0, len(results))
	for _, r := range results {
		items := make([]map[string]any, 0, len(r.Exemplars))
		for _, e := range r.Exemplars {
			items = append(items, map[string]any{
				"labels":    labelSetToMap(e.Labels),
				"value":     e.Value.String(),
				"timestamp": formatTime(e.Timestamp.Time()),
			})
		}
		exemplars = append(exemplars, map[string]any{
			"series_labels": labelSetToMap(r.SeriesLabels),
			"exemplars":     items,
		})
	}

	return common.SuccessResult(map[string]any{
		"exemplars": exemplars,
		"count":     len(exemplars),
	}), nil
}

const exemplarsQuerySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Exemplars Query Parameters",
  "description": "Query exemplars for a PromQL expression.",
  "properties": {
    "query": {
      "type": "string",
      "description": "PromQL expression to evaluate."
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
  },
  "required": ["query", "start", "end"]
}`
