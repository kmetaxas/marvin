package kafka

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*brokerConfigDiffTask)(nil)

type brokerConfigDiffTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *brokerConfigDiffTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *brokerConfigDiffTask) Name() string { return "kafka.broker.config.diff" }

func (t *brokerConfigDiffTask) JSONSchema() string { return brokerConfigDiffSchema }

func (t *brokerConfigDiffTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	brokerIDs, err := OptionalInt32Slice(params, "broker_ids")
	if err != nil {
		return common.TaskFailure(err)
	}
	keys, err := optionalStringSlice(params, "keys")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	// If no broker IDs specified, discover all brokers.
	if len(brokerIDs) == 0 {
		brokers, err := t.currentClient().ListBrokers(ctx)
		if err != nil {
			return common.TaskFailure(err)
		}
		brokerIDs = brokers.NodeIDs()
	}

	configs, err := t.currentClient().DescribeBrokerConfigs(ctx, brokerIDs...)
	if err != nil {
		return common.TaskFailure(err)
	}

	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	// valuesByKey maps config key -> broker ID -> value.
	valuesByKey := make(map[string]map[int32]string)
	for _, rc := range configs {
		if rc.Err != nil {
			continue
		}
		var brokerID int32
		if _, err := fmt.Sscanf(rc.Name, "%d", &brokerID); err != nil {
			continue
		}
		for _, c := range rc.Configs {
			if len(keySet) > 0 && !keySet[c.Key] {
				continue
			}
			if valuesByKey[c.Key] == nil {
				valuesByKey[c.Key] = make(map[int32]string)
			}
			valuesByKey[c.Key][brokerID] = configToEntry(c).Value
		}
	}

	diff := make([]map[string]any, 0)
	for key, values := range valuesByKey {
		// Determine the set of distinct values and outliers.
		distinct := make(map[string]bool)
		for _, v := range values {
			distinct[v] = true
		}
		if len(distinct) <= 1 {
			continue
		}

		counts := make(map[string]int)
		for _, v := range values {
			counts[v]++
		}
		type countPair struct {
			value string
			count int
		}
		pairs := make([]countPair, 0, len(counts))
		for v, n := range counts {
			pairs = append(pairs, countPair{value: v, count: n})
		}
		slices.SortFunc(pairs, func(a, b countPair) int {
			if a.count != b.count {
				return b.count - a.count
			}
			return strings.Compare(a.value, b.value)
		})
		mostCommon := pairs[0].value

		outliers := make([]int32, 0)
		for id, v := range values {
			if v != mostCommon {
				outliers = append(outliers, id)
			}
		}
		slices.Sort(outliers)

		diff = append(diff, map[string]any{
			"key":             key,
			"values":          values,
			"outlier_brokers": outliers,
		})
	}

	sort.Slice(diff, func(i, j int) bool { return diff[i]["key"].(string) < diff[j]["key"].(string) })

	return common.SuccessResult(map[string]any{
		"diff":  diff,
		"count": len(diff),
	}), nil
}

const brokerConfigDiffSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Broker Config Diff Parameters",
  "description": "Compare effective configuration between brokers.",
  "properties": {
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"},
      "description": "Broker IDs to compare. Omit for all."
    },
    "keys": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific keys to compare. Omit for all."
    }
  }
}`
