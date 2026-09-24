package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*metadataGetTask)(nil)

type metadataGetTask struct {
	provider *Provider
}

func (t *metadataGetTask) Name() string { return "prometheus.metadata.get" }

func (t *metadataGetTask) JSONSchema() string { return metadataGetSchema }

func (t *metadataGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

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

	md, err := client.Metadata(ctx, metric, "")
	if err != nil {
		return common.TaskFailure(err)
	}

	metadata := make([]map[string]any, 0, len(md))
	for name, entries := range md {
		for _, e := range entries {
			metadata = append(metadata, map[string]any{
				"metric": name,
				"type":   string(e.Type),
				"help":   e.Help,
				"unit":   e.Unit,
			})
		}
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

const metadataGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Metadata Get Parameters",
  "description": "Get metric metadata (type, help, unit).",
  "properties": {
    "metric": {
      "type": "string",
      "description": "Metric name. Omit to return metadata for all metrics."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of results to return."
    }
  }
}`
