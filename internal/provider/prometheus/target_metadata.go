package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*targetMetadataTask)(nil)

type targetMetadataTask struct {
	provider *Provider
}

func (t *targetMetadataTask) Name() string { return "prometheus.target.metadata" }

func (t *targetMetadataTask) JSONSchema() string { return targetMetadataSchema }

func (t *targetMetadataTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	matchTarget, err := optionalStringMap(params, "match_target")
	if err != nil {
		return common.TaskFailure(err)
	}

	metric, err := common.OptionalString(params, "metric", "")
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

	matchTargetStr := ""
	if len(matchTarget) > 0 {
		matchTargetStr = encodeMatchTarget(matchTarget)
	}

	md, err := client.TargetsMetadata(ctx, matchTargetStr, metric, "")
	if err != nil {
		return common.TaskFailure(err)
	}

	metadata := make([]map[string]any, 0, len(md))
	for _, m := range md {
		metadata = append(metadata, map[string]any{
			"target": m.Target,
			"metric": m.Metric,
			"type":   string(m.Type),
			"help":   m.Help,
			"unit":   m.Unit,
		})
	}

	truncated := false
	if len(metadata) > limit {
		metadata = metadata[:limit]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"metadata":  metadata,
		"count":     len(metadata),
		"truncated": truncated,
	}), nil
}

func encodeMatchTarget(matchTarget map[string]string) string {
	out := ""
	for k, v := range matchTarget {
		if out != "" {
			out += ","
		}
		out += fmt.Sprintf(`%s="%s"`, k, v)
	}
	return "{" + out + "}"
}

const targetMetadataSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Target Metadata Parameters",
  "description": "Get metadata about metrics scraped from targets.",
  "properties": {
    "match_target": {
      "type": "object",
      "additionalProperties": { "type": "string" },
      "description": "Filter by target labels."
    },
    "metric": {
      "type": "string",
      "description": "Filter by metric name."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of results to return."
    }
  }
}`
