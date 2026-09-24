package prometheus

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*queryRangeTask)(nil)

type queryRangeTask struct {
	provider *Provider
}

func (t *queryRangeTask) Name() string { return "prometheus.query.range" }

func (t *queryRangeTask) JSONSchema() string { return queryRangeSchema }

func (t *queryRangeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	stepStr, err := common.OptionalString(params, "step", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	step := guardrails.MinStep
	if stepStr != "" {
		parsed, err := time.ParseDuration(stepStr)
		if err != nil {
			return common.TaskFailure(fmt.Errorf("parameter step must be a duration: %w", err))
		}
		if parsed < guardrails.MinStep {
			return common.TaskFailure(fmt.Errorf("parameter step must be at least %s", guardrails.MinStep))
		}
		step = parsed
	}

	timeoutSeconds, err := common.OptionalInt(params, "timeout_seconds", int(guardrails.QueryTimeout.Seconds()))
	if err != nil {
		return common.TaskFailure(err)
	}
	if timeoutSeconds <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter timeout_seconds must be greater than 0"))
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	timeout = min(timeout, guardrails.QueryTimeout)

	opts := []v1.Option{
		v1.WithTimeout(timeout),
		v1.WithLimit(uint64(guardrails.MaxReturnedSeries)),
	}

	value, warnings, err := client.QueryRange(ctx, query, v1.Range{Start: start, End: end, Step: step}, opts...)
	if err != nil {
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"result_type": value.Type().String(),
		"truncated":   false,
		"warnings":    []string(warnings),
	}

	switch v := value.(type) {
	case model.Matrix:
		mat, truncated := truncateMatrix(v, guardrails.MaxReturnedSeries, guardrails.MaxReturnedSamples)
		items := make([]map[string]any, 0, len(mat))
		for _, ss := range mat {
			items = append(items, sampleStreamToMap(ss))
		}
		result["result"] = items
		result["truncated"] = truncated
	default:
		return common.TaskFailure(fmt.Errorf("unexpected query result type %T", value))
	}

	return common.SuccessResult(result), nil
}

const queryRangeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Range Query Parameters",
  "description": "Execute a PromQL range query over a time interval.",
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
    },
    "step": {
      "type": "string",
      "description": "Query resolution step width in duration format (e.g. '15s', '5m')."
    },
    "timeout_seconds": {
      "type": "integer",
      "minimum": 1,
      "maximum": 300,
      "default": 30,
      "description": "Query timeout in seconds. Capped by provider guardrails."
    }
  },
  "required": ["query", "start", "end"]
}`
