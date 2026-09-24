package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*clusterConfigGetTask)(nil)

type clusterConfigGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *clusterConfigGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *clusterConfigGetTask) Name() string { return "kafka.cluster.config.get" }

func (t *clusterConfigGetTask) JSONSchema() string { return clusterConfigGetSchema }

func (t *clusterConfigGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	keys, err := optionalStringSlice(params, "keys")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	// No broker IDs => cluster-level dynamic/default config.
	configs, err := t.currentClient().DescribeBrokerConfigs(ctx)
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
			entries = append(entries, configToEntry(c))
		}
	}

	return common.SuccessResult(map[string]any{
		"configs": entries,
		"count":   len(entries),
	}), nil
}

const clusterConfigGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Cluster Config Get Parameters",
  "description": "Retrieve cluster-wide dynamic/default broker configuration.",
  "properties": {
    "keys": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific config keys to retrieve. Omit for all."
    }
  }
}`
