package log

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*searchTask)(nil)

type searchTask struct{ provider *Provider }

func (t *searchTask) Name() string { return "log.graylog.search" }

func (t *searchTask) JSONSchema() string { return searchSchema }

func (t *searchTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("log.graylog.search starting", "capability", t.Name())

	query, err := common.OptionalString(params, "query", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	tr, err := parseTimerange(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	fields, err := parseFields(params, []string{"timestamp", "source", "level", "message"})
	if err != nil {
		return common.TaskFailure(err)
	}

	streams, err := parseStreams(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	includeMessages, err := common.OptionalBool(params, "include_messages", true)
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

	limit = guardrails.LimitResults(limit)

	req := searchMessagesRequest{
		Query:     query,
		Streams:   streams,
		Fields:    fields,
		Size:      limit,
		Timerange: tr,
	}

	resp, err := client.SearchMessages(ctx, req)
	if err != nil {
		slog.Info("log.graylog.search failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"total_results": resp.TotalResults,
		"returned":      len(resp.Messages),
		"truncated":     len(resp.Messages) >= limit && resp.TotalResults > limit,
		"time_range":    tr,
		"fields":        fields,
	}

	if includeMessages {
		result["messages"] = resp.Messages
	}

	slog.Info("log.graylog.search succeeded", "capability", t.Name(), "total_results", resp.TotalResults, "returned", len(resp.Messages))
	return common.SuccessResult(result), nil
}

const searchSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "log.graylog.search Parameters",
  "description": "Search Graylog messages with filters and time range.",
  "properties": {
    "query": {
      "type": "string",
      "description": "Graylog search query (Lucene syntax). Example: 'source:web-01 AND level:3'"
    },
    "timerange": {
      "type": "object",
      "description": "Time range specification.",
      "properties": {
        "type": { "type": "string", "enum": ["relative", "absolute", "keyword"] },
        "range": { "type": "integer", "description": "Seconds for relative timerange" },
        "from": { "type": "string", "format": "date-time" },
        "to": { "type": "string", "format": "date-time" },
        "keyword": { "type": "string", "description": "e.g. 'last five minutes'" }
      },
      "required": ["type"]
    },
    "fields": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Fields to return. Default: ['timestamp', 'source', 'level', 'message']"
    },
    "streams": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Stream IDs to search within."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100,
      "description": "Max messages to return."
    },
    "include_messages": {
      "type": "boolean",
      "default": true,
      "description": "If false, return only metadata without messages."
    }
  }
}`
