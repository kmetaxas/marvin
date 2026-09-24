package kafka

import (
	"context"
	"fmt"
	"path"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupListTask)(nil)

type consumerGroupListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupListTask) Name() string { return "kafka.consumer_group.list" }

func (t *consumerGroupListTask) JSONSchema() string { return consumerGroupListSchema }

func (t *consumerGroupListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	states, err := optionalStringSlice(params, "state")
	if err != nil {
		return common.TaskFailure(err)
	}
	pattern, err := common.OptionalString(params, "pattern", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := NormalizeLimitKafka(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	groups, err := t.currentClient().ListGroups(ctx, states...)
	if err != nil {
		return common.TaskFailure(err)
	}

	summaries := make([]ConsumerGroupSummary, 0, len(groups))
	truncated := false
	for _, g := range groups.Sorted() {
		if pattern != "" {
			matched, matchErr := path.Match(pattern, g.Group)
			if matchErr != nil {
				return common.TaskFailure(fmt.Errorf("invalid pattern: %w", matchErr))
			}
			if !matched {
				continue
			}
		}
		if len(summaries) >= limit {
			truncated = true
			break
		}
		summaries = append(summaries, ConsumerGroupSummary{
			GroupID:      g.Group,
			State:        g.State,
			ProtocolType: g.ProtocolType,
		})
	}

	return common.SuccessResult(map[string]any{
		"groups":    summaries,
		"count":     len(summaries),
		"truncated": truncated,
	}), nil
}

const consumerGroupListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group List Parameters",
  "description": "List consumer groups and states.",
  "properties": {
    "state": {
      "type": "array",
      "items": {"type": "string", "enum": ["Unknown", "PreparingRebalance", "CompletingRebalance", "Stable", "Dead", "Empty"]},
      "description": "Filter to specific states."
    },
    "pattern": {
      "type": "string",
      "description": "Glob pattern to filter group IDs."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    },
    "page_token": {
      "type": "string"
    }
  }
}`
