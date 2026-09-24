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

var _ task.Task = (*queryInstantTask)(nil)

type queryInstantTask struct {
	provider *Provider
}

func (t *queryInstantTask) Name() string { return "prometheus.query.instant" }

func (t *queryInstantTask) JSONSchema() string { return queryInstantSchema }

func (t *queryInstantTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	ts := time.Now()
	if timeStr, err := common.OptionalString(params, "time", ""); err != nil {
		return common.TaskFailure(err)
	} else if timeStr != "" {
		parsed, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return common.TaskFailure(fmt.Errorf("parameter time must be RFC3339: %w", err))
		}
		ts = parsed
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

	value, warnings, err := client.Query(ctx, query, ts, opts...)
	if err != nil {
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"result_type": value.Type().String(),
		"truncated":   false,
		"warnings":    []string(warnings),
	}

	switch v := value.(type) {
	case model.Vector:
		vec, truncated := truncateVector(v, guardrails.MaxReturnedSeries, guardrails.MaxReturnedSamples)
		items := make([]map[string]any, 0, len(vec))
		for _, s := range vec {
			items = append(items, sampleToMap(s))
		}
		result["result"] = items
		result["truncated"] = truncated
	case *model.Scalar:
		result["result"] = []any{float64(v.Timestamp) / 1000, v.Value.String()}
	case *model.String:
		result["result"] = []any{float64(v.Timestamp) / 1000, v.Value}
	default:
		return common.TaskFailure(fmt.Errorf("unexpected query result type %T", value))
	}

	return common.SuccessResult(result), nil
}

const queryInstantSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Instant Query Parameters",
  "description": "Execute an instant PromQL query at a specific point in time.",
  "properties": {
    "query": {
      "type": "string",
      "description": "PromQL expression to evaluate."
    },
    "time": {
      "type": "string",
      "format": "date-time",
      "description": "Evaluation timestamp in RFC3339. Omit for 'now'."
    },
    "timeout_seconds": {
      "type": "integer",
      "minimum": 1,
      "maximum": 300,
      "default": 30,
      "description": "Query timeout in seconds. Capped by provider guardrails."
    }
  },
  "required": ["query"]
}`
