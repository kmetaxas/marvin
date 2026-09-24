package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*seriesListTask)(nil)

type seriesListTask struct {
	provider *Provider
}

func (t *seriesListTask) Name() string { return "prometheus.series.list" }

func (t *seriesListTask) JSONSchema() string { return seriesListSchema }

func (t *seriesListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	matches, err := requireStringSlice(params, "match")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	start, end, err := parseOptionalRange(params)
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

	series, _, err := client.Series(ctx, matches, start, end)
	if err != nil {
		return common.TaskFailure(err)
	}

	truncated := false
	if len(series) > limit {
		series = series[:limit]
		truncated = true
	}

	items := make([]map[string]string, 0, len(series))
	for _, ls := range series {
		items = append(items, labelSetToMap(ls))
	}

	return common.SuccessResult(map[string]any{
		"series":    items,
		"count":     len(items),
		"truncated": truncated,
	}), nil
}

const seriesListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Series List Parameters",
  "description": "Find time series matching label selectors.",
  "properties": {
    "match": {
      "type": "array",
      "items": { "type": "string" },
      "minItems": 1,
      "description": "Label matchers, e.g. ['up', 'job=~\"prometheus\"']."
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
      "description": "Maximum number of series to return."
    }
  },
  "required": ["match"]
}`

// requireStringSlice extracts a required []string parameter from the params map.
func requireStringSlice(params map[string]any, key string) ([]string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("missing required parameter: %s", key)
	}
	raw, ok := v.([]string)
	if !ok {
		if iface, ok2 := v.([]any); ok2 {
			out := make([]string, 0, len(iface))
			for _, item := range iface {
				s, ok3 := item.(string)
				if !ok3 {
					return nil, fmt.Errorf("parameter %s must be an array of strings", key)
				}
				out = append(out, s)
			}
			if len(out) == 0 {
				return nil, fmt.Errorf("missing required parameter: %s", key)
			}
			return out, nil
		}
		return nil, fmt.Errorf("parameter %s must be an array of strings", key)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing required parameter: %s", key)
	}
	return raw, nil
}

// parseOptionalRange parses optional start/end RFC3339 params. Zero times are
// returned when omitted.
func parseOptionalRange(params map[string]any) (time.Time, time.Time, error) {
	var start, end time.Time
	startStr, err := common.OptionalString(params, "start", "")
	if err != nil {
		return start, end, err
	}
	endStr, err := common.OptionalString(params, "end", "")
	if err != nil {
		return start, end, err
	}
	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return start, end, fmt.Errorf("parameter start must be RFC3339: %w", err)
		}
	}
	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return start, end, fmt.Errorf("parameter end must be RFC3339: %w", err)
		}
	}
	return start, end, nil
}
