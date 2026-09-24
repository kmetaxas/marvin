package kafka

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*topicConfigDiffTask)(nil)

type topicConfigDiffTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *topicConfigDiffTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *topicConfigDiffTask) Name() string { return "kafka.topic.config.diff" }

func (t *topicConfigDiffTask) JSONSchema() string { return topicConfigDiffSchema }

func (t *topicConfigDiffTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	if len(topics) == 0 {
		return common.TaskFailure(fmt.Errorf("missing required parameter: topics"))
	}
	keys, err := optionalStringSlice(params, "keys")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	configs, err := t.currentClient().DescribeTopicConfigs(ctx, topics...)
	if err != nil {
		return common.TaskFailure(err)
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[k] = struct{}{}
	}

	// valuesByKey maps config key -> topic -> value.
	valuesByKey := make(map[string]map[string]string)
	for _, rc := range configs {
		if rc.Err != nil {
			continue
		}
		for _, c := range rc.Configs {
			if len(keySet) > 0 {
				if _, ok := keySet[c.Key]; !ok {
					continue
				}
			}
			if valuesByKey[c.Key] == nil {
				valuesByKey[c.Key] = make(map[string]string)
			}
			valuesByKey[c.Key][rc.Name] = configToEntry(c).Value
		}
	}

	diff := make([]map[string]any, 0)
	for key, values := range valuesByKey {
		distinct := make(map[string]struct{})
		for _, v := range values {
			distinct[v] = struct{}{}
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
			if a.value < b.value {
				return -1
			}
			if a.value > b.value {
				return 1
			}
			return 0
		})
		mostCommon := pairs[0].value

		anomalous := make([]string, 0)
		for topic, v := range values {
			if v != mostCommon {
				anomalous = append(anomalous, topic)
			}
		}
		sort.Strings(anomalous)

		diff = append(diff, map[string]any{
			"key":              key,
			"values":           values,
			"anomalous_topics": anomalous,
		})
	}

	sort.Slice(diff, func(i, j int) bool { return diff[i]["key"].(string) < diff[j]["key"].(string) })

	return common.SuccessResult(map[string]any{
		"diff":  diff,
		"count": len(diff),
	}), nil
}

const topicConfigDiffSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Topic Config Diff Parameters",
  "description": "Compare configuration of several topics.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Topics to compare."
    },
    "keys": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific keys to compare. Omit for all."
    }
  },
  "required": ["topics"]
}`
