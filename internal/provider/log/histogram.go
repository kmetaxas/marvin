package log

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*histogramTask)(nil)

type histogramTask struct{ provider *Provider }

func (t *histogramTask) Name() string { return "log.graylog.histogram" }

func (t *histogramTask) JSONSchema() string { return histogramSchema }

func (t *histogramTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("log.graylog.histogram starting", "capability", t.Name())

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

	iv, err := parseInterval(params, 5, "m")
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
		Query:     query,
		Streams:   streams,
		GroupBy:   []groupBy{{Field: "timestamp", Interval: iv}},
		Metrics:   []metric{{Function: "count"}},
		Timerange: tr,
	}

	resp, err := client.SearchAggregate(ctx, req)
	if err != nil {
		slog.Info("log.graylog.histogram failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	buckets := make([]map[string]any, 0, len(resp.Rows))
	totalCount := 0
	for _, row := range resp.Rows {
		cnt := makeCount(row)
		totalCount += cnt
		buckets = append(buckets, map[string]any{
			"timestamp": makeTimestamp(row),
			"count":     cnt,
		})
	}

	result := map[string]any{
		"total_count": totalCount,
		"buckets":     buckets,
		"interval":    iv,
		"time_range":  tr,
	}

	slog.Info("log.graylog.histogram succeeded", "capability", t.Name(), "total_count", totalCount, "buckets", len(buckets))
	return common.SuccessResult(result), nil
}

const histogramSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "log.graylog.histogram Parameters",
  "description": "Return time-bucketed hit counts for a query.",
  "properties": {
    "query": {
      "type": "string",
      "description": "Graylog search query."
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
    },
    "interval": {
      "type": "object",
      "description": "Time bucket interval. Default: 5 minutes.",
      "properties": {
        "value": { "type": "integer", "minimum": 1, "default": 5 },
        "unit": { "type": "string", "enum": ["s", "m", "h", "d", "w", "M"], "default": "m" }
      }
    }
  }
}`
