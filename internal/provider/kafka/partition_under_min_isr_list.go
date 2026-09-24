package kafka

import (
	"context"
	"fmt"
	"strconv"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionUnderMinISRListTask)(nil)

type partitionUnderMinISRListTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionUnderMinISRListTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionUnderMinISRListTask) Name() string { return "kafka.partition.under_min_isr.list" }

func (t *partitionUnderMinISRListTask) JSONSchema() string { return partitionUnderMinISRListSchema }

func (t *partitionUnderMinISRListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
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

	details, err := t.currentClient().ListTopics(ctx, topics...)
	if err != nil {
		return common.TaskFailure(err)
	}

	// Determine min.insync.replicas per topic from topic configs.
	topicNames := details.Names()
	minISRByTopic := make(map[string]int, len(topicNames))
	if len(topicNames) > 0 {
		configs, err := t.currentClient().DescribeTopicConfigs(ctx, topicNames...)
		if err != nil {
			return common.TaskFailure(err)
		}
		for _, rc := range configs {
			if rc.Err != nil {
				continue
			}
			for _, c := range rc.Configs {
				if c.Key != "min.insync.replicas" {
					continue
				}
				if n, err := strconv.Atoi(c.MaybeValue()); err == nil {
					minISRByTopic[rc.Name] = n
				}
			}
		}
	}

	underMinISR := make([]PartitionInfo, 0)
	truncated := false
	for _, td := range details.Sorted() {
		minISR, ok := minISRByTopic[td.Topic]
		if !ok {
			continue
		}
		for _, pd := range td.Partitions.Sorted() {
			if len(pd.ISR) >= minISR {
				continue
			}
			if len(underMinISR) >= limit {
				truncated = true
				break
			}
			underMinISR = append(underMinISR, partitionToInfo(pd))
		}
		if truncated {
			break
		}
	}

	return common.SuccessResult(map[string]any{
		"under_min_isr_partitions": underMinISR,
		"count":                    len(underMinISR),
		"truncated":                truncated,
	}), nil
}

const partitionUnderMinISRListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Under-Min-ISR Partition List Parameters",
  "description": "Find partitions currently below configured min.insync.replicas.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"}
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`
