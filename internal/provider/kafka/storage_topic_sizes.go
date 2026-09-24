package kafka

import (
	"context"
	"fmt"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*storageTopicSizesTask)(nil)

type storageTopicSizesTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *storageTopicSizesTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *storageTopicSizesTask) Name() string { return "kafka.storage.topic_sizes.get" }

func (t *storageTopicSizesTask) JSONSchema() string { return storageTopicSizesSchema }

func (t *storageTopicSizesTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	brokerIDs, err := OptionalInt32Slice(params, "broker_ids")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	all, err := t.currentClient().DescribeAllLogDirs(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	brokers := brokerIDSet(brokerIDs)
	topicsSet := topicSet(topics)

	// Aggregate per topic: total size, per-broker size, partition count.
	type brokerSize struct {
		brokerID int32
		size     int64
	}
	type topicAgg struct {
		totalSize  int64
		byBroker   map[int32]int64
		partitions int
	}
	aggs := make(map[string]*topicAgg)

	for _, d := range sortedLogDirs(all, brokers) {
		for topic, partitions := range d.Topics {
			if !topicMatches(topicsSet, topic) {
				continue
			}
			for _, p := range partitions {
				agg, ok := aggs[topic]
				if !ok {
					agg = &topicAgg{byBroker: make(map[int32]int64)}
					aggs[topic] = agg
				}
				agg.totalSize += p.Size
				agg.byBroker[d.Broker] += p.Size
				agg.partitions++
			}
		}
	}

	// Deterministic output: sort topic names.
	names := make([]string, 0, len(aggs))
	for name := range aggs {
		names = append(names, name)
	}
	sort.Strings(names)

	topicSizes := make([]map[string]any, 0, len(names))
	for _, name := range names {
		agg := aggs[name]
		byBroker := make([]map[string]any, 0, len(agg.byBroker))
		brokerIDsSorted := make([]int32, 0, len(agg.byBroker))
		for id := range agg.byBroker {
			brokerIDsSorted = append(brokerIDsSorted, id)
		}
		sort.Slice(brokerIDsSorted, func(i, j int) bool { return brokerIDsSorted[i] < brokerIDsSorted[j] })
		for _, id := range brokerIDsSorted {
			byBroker = append(byBroker, map[string]any{
				"broker_id":  id,
				"size_bytes": agg.byBroker[id],
			})
		}
		topicSizes = append(topicSizes, map[string]any{
			"topic":            name,
			"total_size_bytes": agg.totalSize,
			"by_broker":        byBroker,
			"partition_count":  agg.partitions,
		})
	}

	return common.SuccessResult(map[string]any{
		"topic_sizes": topicSizes,
		"count":       len(topicSizes),
	}), nil
}

const storageTopicSizesSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Storage Topic Sizes Parameters",
  "description": "Aggregate partition sizes into topic/broker totals.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics."
    },
    "broker_ids": {
      "type": "array",
      "items": {"type": "integer"},
      "description": "Filter to specific brokers."
    }
  }
}`
