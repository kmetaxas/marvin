package log

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*countTask)(nil)

type countTask struct{ provider *Provider }

func (t *countTask) Name() string { return "log.graylog.count" }

func (t *countTask) JSONSchema() string { return countSchema }

func (t *countTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("log.graylog.count starting", "capability", t.Name())

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
		Query:     query,
		Streams:   streams,
		Metrics:   []metric{{Function: "count"}},
		Timerange: tr,
	}

	resp, err := client.SearchAggregate(ctx, req)
	if err != nil {
		slog.Info("log.graylog.count failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	count := 0
	if len(resp.Rows) > 0 {
		count = makeCount(resp.Rows[0])
	}

	result := map[string]any{
		"count":      count,
		"time_range": tr,
		"query":      query,
	}

	slog.Info("log.graylog.count succeeded", "capability", t.Name(), "count", count)
	return common.SuccessResult(result), nil
}

const countSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "log.graylog.count Parameters",
  "description": "Return the number of matching log messages without retrieving them.",
  "properties": {
    "query": {
      "type": "string",
      "description": "Graylog search query."
    },
    "timerange": {
      "type": "object",
      "description": "Time range specification. Same schema as log.graylog.search.",
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
  }
}`
