package prometheus

import (
	"context"
	"fmt"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*metricNamesTask)(nil)

type metricNamesTask struct {
	provider *Provider
}

func (t *metricNamesTask) Name() string { return "prometheus.metric.names" }

func (t *metricNamesTask) JSONSchema() string { return metricNamesSchema }

func (t *metricNamesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	search, err := common.OptionalString(params, "search", "")
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

	values, _, err := client.LabelValues(ctx, "__name__", matches, start, end)
	if err != nil {
		return common.TaskFailure(err)
	}

	names := make([]string, 0, len(values))
	for _, v := range values {
		name := string(v)
		if search != "" && !strings.Contains(name, search) {
			continue
		}
		names = append(names, name)
	}

	truncated := false
	if len(names) > limit {
		names = names[:limit]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"names":     names,
		"count":     len(names),
		"truncated": truncated,
	}), nil
}

const metricNamesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Metric Names Parameters",
  "description": "List available metric names.",
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
    },
    "search": {
      "type": "string",
      "description": "Substring filter applied to metric names."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of names to return."
    }
  }
}`

// optionalStringSlice extracts an optional []string parameter from the params map.
func optionalStringSlice(params map[string]any, key string) ([]string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return nil, nil
	}
	if raw, ok := v.([]string); ok {
		return raw, nil
	}
	if iface, ok := v.([]any); ok {
		out := make([]string, 0, len(iface))
		for _, item := range iface {
			s, ok2 := item.(string)
			if !ok2 {
				return nil, fmt.Errorf("parameter %s must be an array of strings", key)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, fmt.Errorf("parameter %s must be an array of strings", key)
}
