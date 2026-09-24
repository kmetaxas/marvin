package kafka

import (
	"context"
	"fmt"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupLagGetTask)(nil)

type consumerGroupLagGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupLagGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupLagGetTask) Name() string { return "kafka.consumer_group.lag.get" }

func (t *consumerGroupLagGetTask) JSONSchema() string { return consumerGroupLagGetSchema }

func (t *consumerGroupLagGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	groupID, err := common.RequireString(params, "group_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	aggregate, err := common.OptionalString(params, "aggregate", "none")
	if err != nil {
		return common.TaskFailure(err)
	}
	switch aggregate {
	case "none", "topic", "group":
	default:
		return common.TaskFailure(fmt.Errorf("parameter aggregate must be one of: none, topic, group"))
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	lags, err := t.currentClient().Lag(ctx, groupID)
	if err != nil {
		return common.TaskFailure(err)
	}

	groupLag, ok := lags[groupID]
	if !ok {
		return common.TaskFailure(fmt.Errorf("group %s not found", groupID))
	}
	if groupLag.Error() != nil {
		return common.TaskFailure(groupLag.Error())
	}

	topicFilter := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		topicFilter[topic] = struct{}{}
	}

	perPartition := make([]map[string]any, 0)
	for _, l := range groupLag.Lag.Sorted() {
		if len(topicFilter) > 0 {
			if _, ok := topicFilter[l.Topic]; !ok {
				continue
			}
		}
		perPartition = append(perPartition, map[string]any{
			"topic":            l.Topic,
			"partition":        l.Partition,
			"committed_offset": l.Commit.At,
			"log_end_offset":   l.End.Offset,
			"lag":              l.Lag,
		})
	}

	switch aggregate {
	case "topic":
		return common.SuccessResult(lagByTopic(groupID, perPartition)), nil
	case "group":
		return common.SuccessResult(lagForGroup(groupID, perPartition)), nil
	default:
		return common.SuccessResult(map[string]any{
			"lag":   perPartition,
			"count": len(perPartition),
		}), nil
	}
}

func lagByTopic(groupID string, perPartition []map[string]any) map[string]any {
	type topicAgg struct {
		totalLag        int64
		partitionCount  int
		maxLag          int64
		maxLagPartition int32
	}
	byTopic := make(map[string]*topicAgg)
	for _, p := range perPartition {
		topic := p["topic"].(string)
		lag := p["lag"].(int64)
		partition := p["partition"].(int32)
		agg, ok := byTopic[topic]
		if !ok {
			agg = &topicAgg{maxLagPartition: -1}
			byTopic[topic] = agg
		}
		if lag > 0 {
			agg.totalLag += lag
		}
		agg.partitionCount++
		if lag > agg.maxLag {
			agg.maxLag = lag
			agg.maxLagPartition = partition
		}
	}

	names := make([]string, 0, len(byTopic))
	for name := range byTopic {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]map[string]any, 0, len(names))
	for _, name := range names {
		agg := byTopic[name]
		entries = append(entries, map[string]any{
			"topic":             name,
			"total_lag":         agg.totalLag,
			"partition_count":   agg.partitionCount,
			"max_lag_partition": agg.maxLagPartition,
			"max_lag":           agg.maxLag,
		})
	}

	return map[string]any{
		"lag_by_topic": entries,
		"count":        len(entries),
	}
}

func lagForGroup(groupID string, perPartition []map[string]any) map[string]any {
	var totalLag int64
	var maxLag int64
	topicSet := make(map[string]struct{})
	for _, p := range perPartition {
		lag := p["lag"].(int64)
		if lag > 0 {
			totalLag += lag
		}
		if lag > maxLag {
			maxLag = lag
		}
		topicSet[p["topic"].(string)] = struct{}{}
	}
	return map[string]any{
		"group_id":    groupID,
		"total_lag":   totalLag,
		"topic_count": len(topicSet),
		"max_lag":     maxLag,
	}
}

const consumerGroupLagGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group Lag Get Parameters",
  "description": "Calculate committed offset vs log-end offset per partition.",
  "properties": {
    "group_id": {
      "type": "string"
    },
    "topics": {
      "type": "array",
      "items": {"type": "string"}
    },
    "aggregate": {
      "type": "string",
      "enum": ["none", "topic", "group"],
      "default": "none",
      "description": "Aggregation level for lag reporting."
    }
  },
  "required": ["group_id"]
}`
