package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var _ task.Task = (*topicConfigGetTask)(nil)

type topicConfigGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *topicConfigGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *topicConfigGetTask) Name() string { return "kafka.topic.config.get" }

func (t *topicConfigGetTask) JSONSchema() string { return topicConfigGetSchema }

func (t *topicConfigGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.RequireString(params, "topic")
	if err != nil {
		return common.TaskFailure(err)
	}
	keys, err := optionalStringSlice(params, "keys")
	if err != nil {
		return common.TaskFailure(err)
	}
	includeDefaults, err := common.OptionalBool(params, "include_defaults", true)
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	configs, err := t.currentClient().DescribeTopicConfigs(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}

	rc, err := configs.On(topic, nil)
	if err != nil {
		return common.TaskFailure(err)
	}
	if rc.Err != nil {
		return common.TaskFailure(rc.Err)
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[k] = struct{}{}
	}

	entries := make([]ConfigEntry, 0, len(rc.Configs))
	for _, c := range rc.Configs {
		if len(keySet) > 0 {
			if _, ok := keySet[c.Key]; !ok {
				continue
			}
		}
		if !includeDefaults && c.Source == kmsg.ConfigSourceDefaultConfig {
			continue
		}
		entries = append(entries, configToEntry(c))
	}

	return common.SuccessResult(map[string]any{
		"topic":   topic,
		"configs": entries,
		"count":   len(entries),
	}), nil
}

const topicConfigGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Topic Config Get Parameters",
  "description": "Get effective topic configuration.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Topic name."
    },
    "keys": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific config keys. Omit for all."
    },
    "include_defaults": {
      "type": "boolean",
      "default": true,
      "description": "Include default values."
    }
  },
  "required": ["topic"]
}`
