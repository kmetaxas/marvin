package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*brokerConfigGetTask)(nil)

type brokerConfigGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *brokerConfigGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *brokerConfigGetTask) Name() string { return "kafka.broker.config.get" }

func (t *brokerConfigGetTask) JSONSchema() string { return brokerConfigGetSchema }

func (t *brokerConfigGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	brokerID, err := RequireInt32(params, "broker_id")
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

	configs, err := t.currentClient().DescribeBrokerConfigs(ctx, brokerID)
	if err != nil {
		return common.TaskFailure(err)
	}

	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	entries := make([]ConfigEntry, 0)
	for _, rc := range configs {
		if rc.Err != nil {
			return common.TaskFailure(rc.Err)
		}
		for _, c := range rc.Configs {
			if len(keySet) > 0 && !keySet[c.Key] {
				continue
			}
			if !includeDefaults && c.Source.String() == "DEFAULT_CONFIG" {
				continue
			}
			entries = append(entries, configToEntry(c))
		}
	}

	return common.SuccessResult(map[string]any{
		"configs": entries,
		"count":   len(entries),
	}), nil
}

const brokerConfigGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Broker Config Get Parameters",
  "description": "Read effective broker configuration.",
  "properties": {
    "broker_id": {
      "type": "integer",
      "description": "Broker ID to query."
    },
    "keys": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific config keys to retrieve. Omit for all."
    },
    "include_defaults": {
      "type": "boolean",
      "default": true,
      "description": "Include default values in results."
    }
  },
  "required": ["broker_id"]
}`
