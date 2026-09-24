package log

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*fieldstatsTask)(nil)

type fieldstatsTask struct{ provider *Provider }

func (t *fieldstatsTask) Name() string { return "log.graylog.fieldstats" }

func (t *fieldstatsTask) JSONSchema() string { return fieldstatsSchema }

func (t *fieldstatsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("log.graylog.fieldstats starting", "capability", t.Name())

	field, err := common.RequireString(params, "field")
	if err != nil {
		return common.TaskFailure(err)
	}

	query, err := common.OptionalString(params, "query", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	tr, err := parseTimerange(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	streams, err := parseStreams(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("graylog client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	if err := guardrails.ValidateTimerange(tr); err != nil {
		return common.TaskFailure(err)
	}

	req := searchAggregateRequest{
		Query:   query,
		Streams: streams,
		GroupBy: []groupBy{{Field: field}},
		Metrics: []metric{
			{Function: "count"},
			{Function: "min", Field: field},
			{Function: "max", Field: field},
			{Function: "avg", Field: field},
			{Function: "sum", Field: field},
		},
		Timerange: tr,
	}

	resp, err := client.SearchAggregate(ctx, req)
	if err != nil {
		slog.Info("log.graylog.fieldstats failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	stats := map[string]any{
		"count": 0,
		"min":   nil,
		"max":   nil,
		"avg":   nil,
		"sum":   nil,
	}

	if len(resp.Rows) > 0 {
		row := resp.Rows[0]
		for k, v := range row.Values {
			stats[k] = v
		}
	}

	result := map[string]any{
		"field":      field,
		"stats":      stats,
		"time_range": tr,
	}

	slog.Info("log.graylog.fieldstats succeeded", "capability", t.Name(), "field", field)
	return common.SuccessResult(result), nil
}

const fieldstatsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "log.graylog.fieldstats Parameters",
  "description": "Return descriptive statistics for a numeric field.",
  "properties": {
    "query": {
      "type": "string",
      "description": "Graylog search query."
    },
    "field": {
      "type": "string",
      "description": "Field name to compute statistics on. Required."
    },
    "timerange": {
      "type": "object",
      "description": "Time range specification.",
      "properties": {
        "type": { "type": "string", "enum": ["relative", "absolute", "keyword"] },
        "range": { "type": "integer" },
        "from": { "type": "string", "format": "date-time" },
        "to": { "type": "string", "format": "date-time" },
        "keyword": { "type": "string" }
      },
      "required": ["type"]
    },
    "streams": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": ["field"]
}`
