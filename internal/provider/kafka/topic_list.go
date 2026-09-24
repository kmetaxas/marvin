package kafka

import (
	"context"
	"fmt"
	"path"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*topicListTask)(nil)

type topicListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *topicListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *topicListTask) Name() string { return "kafka.topic.list" }

func (t *topicListTask) JSONSchema() string { return topicListSchema }

func (t *topicListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	includeInternal, err := common.OptionalBool(params, "include_internal", false)
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
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	topics, err := t.currentClient().ListTopics(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	summaries := make([]TopicSummary, 0)
	truncated := false
	for _, td := range topics.Sorted() {
		if td.IsInternal && !includeInternal {
			continue
		}
		if pattern != "" {
			matched, matchErr := path.Match(pattern, td.Topic)
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
		summaries = append(summaries, TopicSummary{
			Name:       td.Topic,
			Internal:   td.IsInternal,
			Partitions: len(td.Partitions),
		})
	}

	return common.SuccessResult(map[string]any{
		"topics":          summaries,
		"count":           len(summaries),
		"truncated":       truncated,
		"next_page_token": "",
	}), nil
}

const topicListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Topic List Parameters",
  "description": "List topics, optionally filtering internal topics.",
  "properties": {
    "include_internal": {
      "type": "boolean",
      "default": false,
      "description": "Include internal topics (__consumer_offsets, etc.)."
    },
    "pattern": {
      "type": "string",
      "description": "Glob pattern to filter topic names (e.g. 'orders*')."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100,
      "description": "Maximum topics to return."
    },
    "page_token": {
      "type": "string",
      "description": "Opaque pagination token from previous response."
    }
  }
}`
